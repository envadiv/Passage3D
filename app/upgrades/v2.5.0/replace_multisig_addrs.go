package v2_5

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
)

type AddressMigration struct {
	OldAddress string
	NewAddress string
}

var migrations = []AddressMigration{
	{
		"pasg19zkz9x0u84a0ykzm8lggwy5x0s0gcnwskark5c",
		"",
	},
	{
		"pasg1sap5junfzydgqcll4ezyl4sh4yeekl64qkna34",
		"",
	},
	{
		"pasg1lel0s624jr9zsz4ml6yv9e5r4uzukfs7hwh22w",
		"",
	},
	{
		"pasg1vl7u3a9p37ajemv7wyvuegh7mhujtmdvpt8apu",
		"",
	},
	{
		"pasg1pfdyn3nzajult9e6s2nvhmlgaeglh8lljdv779",
		"",
	},
}

func MigrateMultisigAddresses(
	ctx sdk.Context,
	dk distribution.Keeper,
	bk bank.Keeper,
	ak auth.AccountKeeper,
	sk staking.Keeper,
	azk authz.Keeper,
	ck claim.Keeper,
) error {
	for _, m := range migrations {
		oldAddr, err := sdk.AccAddressFromBech32(m.OldAddress)
		if err != nil {
			return fmt.Errorf("error: %w, invalid bech32 old address: %s", err, m.OldAddress)
		}

		newAddr, err := sdk.AccAddressFromBech32(m.NewAddress)
		if err != nil {
			return fmt.Errorf("error: %w, invalid bech32 new address: %s", err, m.NewAddress)
		}

		oldAccount := ak.GetAccount(ctx, oldAddr)
		if oldAccount == nil {
			return fmt.Errorf("old account %s not found", m.OldAddress)
		}

		newAccount := ak.GetAccount(ctx, newAddr)
		if newAccount == nil {
			return fmt.Errorf("new account %s not found", m.OldAddress)
		}

		switch oldAcc := oldAccount.(type) {
		case *vestingtypes.PeriodicVestingAccount:
			vestingPeriods := oldAcc.VestingPeriods

			// unlock old account vesting periods
			newVestingPeriods := make([]vestingtypes.Period, len(oldAcc.VestingPeriods))
			for i, vp := range oldAcc.VestingPeriods {
				vp.Length = 0
				newVestingPeriods[i] = vp
			}
			oldAcc.VestingPeriods = newVestingPeriods
			ak.SetAccount(ctx, oldAcc)

			// update new account with vesting periods
			newAcc := vestingtypes.NewPeriodicVestingAccount(
				authtypes.NewBaseAccount(newAccount.GetAddress(), newAccount.GetPubKey(),
					newAccount.GetAccountNumber(), newAccount.GetSequence()),
				oldAcc.OriginalVesting, oldAcc.StartTime, vestingPeriods,
			)
			ak.SetAccount(ctx, newAcc)

		case *authtypes.BaseAccount:
			newAcc := authtypes.NewBaseAccount(newAccount.GetAddress(), newAccount.GetPubKey(),
				newAccount.GetAccountNumber(), newAccount.GetSequence())
			ak.SetAccount(ctx, newAcc)
		}

		// send spendable balance from old account to new account
		spendable := bk.SpendableCoins(ctx, oldAccount.GetAddress())
		if err := bk.SendCoins(ctx, oldAccount.GetAddress(), newAccount.GetAddress(), spendable); err != nil {
			return err
		}

		// update delegations, unbond and delegate from new address
		delegations := sk.GetAllDelegatorDelegations(ctx, oldAccount.GetAddress())
		for _, delegation := range delegations {
			validator, ok := sk.GetValidator(ctx, delegation.GetValidatorAddr())
			if ok {
				return fmt.Errorf("validator not found: %s from a delegation with delegator address: %s",
					delegation.ValidatorAddress, delegation.DelegatorAddress)
			}
			amount, err := sk.Unbond(ctx, delegation.GetDelegatorAddr(), delegation.GetValidatorAddr(), delegation.GetShares())
			if err != nil {
				return err
			}

			_, err = sk.Delegate(ctx, newAccount.GetAddress(), amount, validator.GetStatus(), validator, false)
			if err != nil {
				return err
			}
		}

		// update unbonding delegations
		sk.IterateDelegatorUnbondingDelegations(ctx, oldAccount.GetAddress(),
			func(ubd stakingtypes.UnbondingDelegation) (stop bool) {
				// remove old record
				sk.RemoveUnbondingDelegation(ctx, ubd)

				// update record with new address and add again
				ubd.DelegatorAddress = m.NewAddress
				sk.SetUnbondingDelegation(ctx, ubd)

				return false
			})

	}

	return nil
}
