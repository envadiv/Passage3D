package v2_5

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type AddressMigration struct {
	OldAddress string `json:"old_address"`
	NewAddress string `json:"new_address"`
}

type AddressMap map[string]string

type OldDelegation struct {
	Delegation       stakingtypes.Delegation
	DelegationAmount math.Int
}

// MigrateMultisigAddresses migrates account, balance, and delegation state from the
// old multisig addresses to the new ones. This is a historical handler (v2.5.0) that
// already executed on Passage mainnet; it only re-runs on a full genesis replay.
//
// TODO(passage v3.1.0): verify under 0.50 - historical handler, only runs on genesis
// replay. The original 0.47 handler also migrated x/authz grants, x/feegrant
// allowances, and x/gov votes for the migrated addresses. Under the unified SDK 0.50
// upgrade-handler signature the authz and feegrant keepers are no longer threaded to
// this handler, and the 0.50 x/gov keeper dropped GetAllVotes/SetVote (votes are now a
// collections.Map). Those three migration steps are intentionally omitted here. The
// supervisor must confirm none of the five migrated multisig addresses held authz
// grants / feegrant allowances / open gov votes at the v2.5.0 height before relying on
// a from-genesis replay (account/balance/delegation migration below is preserved).
func MigrateMultisigAddresses(
	ctx sdk.Context,
	migrations []AddressMigration,
	bk bank.Keeper,
	ak auth.AccountKeeper,
	sk *staking.Keeper,
	gk govkeeper.Keeper,
) error {
	addressMap := AddressMap{}

	for _, m := range migrations {
		oldAddr, err := sdk.AccAddressFromBech32(m.OldAddress)
		if err != nil {
			return fmt.Errorf("invalid bech32 old address: %s, error: %w", m.OldAddress, err)
		}

		oldAccount := ak.GetAccount(ctx, oldAddr)
		if oldAccount == nil {
			return fmt.Errorf("old account %s not found", m.OldAddress)
		}

		newAddr, err := sdk.AccAddressFromBech32(m.NewAddress)
		if err != nil {
			return fmt.Errorf("invalid bech32 new address: %s, error: %w", m.NewAddress, err)
		}
		addressMap[m.OldAddress] = newAddr.String()

		if err := migrateAccount(ctx, ak, oldAccount, newAddr); err != nil {
			return fmt.Errorf("failed to migrate account: %w", err)
		}

		// unbond old delegation
		delegations, err := unbondOldDelegations(ctx, sk, oldAddr)
		if err != nil {
			return fmt.Errorf("failed to unbond old delegations: %w", err)
		}

		// send spendable balance from old account to new account
		if err := migrateBalances(ctx, bk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate balances: %w", err)
		}

		if err := migrateDelegations(ctx, sk, newAddr, delegations); err != nil {
			return fmt.Errorf("failed to migrate delegations: %w", err)
		}

		// TODO(passage v3.1.0): verify under 0.50 - historical handler, only runs on
		// genesis replay. authz grant migration omitted (authz keeper not threaded to
		// the 0.50 handler signature).

		// TODO(passage v3.1.0): verify under 0.50 - historical handler, only runs on
		// genesis replay. feegrant allowance migration omitted (feegrant keeper not
		// threaded to the 0.50 handler signature).

		// transfer remaining vested tokens from old to new account
		oldAccount = ak.GetAccount(ctx, oldAddr)
		oldAcc, ok := oldAccount.(*vestingtypes.PeriodicVestingAccount)
		if ok && !oldAcc.DelegatedVesting.Empty() {
			newAccount := ak.GetAccount(ctx, newAddr)
			newAcc, _ := newAccount.(*vestingtypes.PeriodicVestingAccount)
			newAcc.DelegatedVesting = newAcc.DelegatedVesting.Add(oldAcc.DelegatedVesting...)
			ak.SetAccount(ctx, newAcc)
			oldAcc.DelegatedVesting = sdk.NewCoins()
			ak.SetAccount(ctx, oldAcc)
		}

		// send again spendable balance from old account to new account to avoid missing balances
		if err := migrateBalances(ctx, bk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate balances: %w", err)
		}
	}

	// TODO(passage v3.1.0): verify under 0.50 - historical handler, only runs on genesis
	// replay. gov vote migration omitted: 0.50 x/gov dropped GetAllVotes/SetVote (votes
	// are a collections.Map keyed by (proposalID, voter); there is no supported in-place
	// voter-rewrite). gk is retained in the signature for parity but unused.
	_ = gk
	_ = addressMap

	return nil
}

// Migrate account information based on type
func migrateAccount(ctx sdk.Context, ak auth.AccountKeeper, oldAccount sdk.AccountI,
	newAddr sdk.AccAddress,
) error {
	switch oldAcc := oldAccount.(type) {
	case *vestingtypes.PeriodicVestingAccount:
		return migrateVestingAccount(ctx, ak, oldAcc, newAddr)

	case *authtypes.BaseAccount:
		return migrateBaseAccount(ctx, ak, newAddr)

	default:
		return fmt.Errorf("not supported account type for upgrade")
	}
}

// Migrate vesting account
func migrateVestingAccount(ctx sdk.Context, ak auth.AccountKeeper, oldAcc *vestingtypes.PeriodicVestingAccount,
	newAddr sdk.AccAddress,
) error {
	// copy old account to new vesting account via proto round-trip
	accBytes, err := oldAcc.Marshal()
	if err != nil {
		return err
	}

	var newVestingAccount vestingtypes.PeriodicVestingAccount
	if err := newVestingAccount.Unmarshal(accBytes); err != nil {
		return err
	}

	// update base account details
	newVestingAccount.BaseAccount = authtypes.NewBaseAccountWithAddress(newAddr)
	newVestingAccount.DelegatedFree = sdk.NewCoins()
	newVestingAccount.DelegatedVesting = sdk.NewCoins()
	ak.SetAccount(ctx, &newVestingAccount)

	// Clear old account's vesting periods
	for i := range oldAcc.VestingPeriods {
		oldAcc.VestingPeriods[i].Length = 0
	}
	// set start time earlier than context time
	oldAcc.StartTime = ctx.BlockTime().Unix() - 10
	oldAcc.EndTime = ctx.BlockTime().Unix()
	ak.SetAccount(ctx, oldAcc)

	return nil
}

// Migrate base account
func migrateBaseAccount(ctx sdk.Context, ak auth.AccountKeeper, newAddr sdk.AccAddress) error {
	newBaseAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	ak.SetAccount(ctx, newBaseAcc)
	return nil
}

func unbondOldDelegations(ctx sdk.Context, sk *staking.Keeper,
	oldAddr sdk.AccAddress,
) ([]OldDelegation, error) {
	// complete all existing redelegations
	if err := sk.IterateDelegatorRedelegations(ctx, oldAddr, func(red stakingtypes.Redelegation) (stop bool) {
		// set all entry completionTime to now so we can complete re-delegation
		redelegationSrc, _ := sdk.ValAddressFromBech32(red.ValidatorSrcAddress)
		redelegationDst, _ := sdk.ValAddressFromBech32(red.ValidatorDstAddress)

		blockTime := ctx.BlockTime()
		for i := range red.Entries {
			red.Entries[i].CompletionTime = blockTime
		}
		if err := sk.SetRedelegation(ctx, red); err != nil {
			panic(err)
		}
		_, err := sk.CompleteRedelegation(ctx, oldAddr, redelegationSrc, redelegationDst)
		if err != nil {
			panic(err)
		}

		return false
	}); err != nil {
		return nil, err
	}

	delegations, err := sk.GetAllDelegatorDelegations(ctx, oldAddr)
	if err != nil {
		return nil, err
	}
	oldDelegations := []OldDelegation{}
	for _, delegation := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(delegation.GetValidatorAddr())
		if err != nil {
			return oldDelegations, err
		}
		shares := delegation.GetShares()

		validator, err := sk.GetValidator(ctx, valAddr)
		if err != nil {
			return oldDelegations, fmt.Errorf("validator not found: %s from delegation %s",
				delegation.ValidatorAddress, delegation.DelegatorAddress)
		}

		delegatedAmount := validator.TokensFromShares(shares).TruncateInt()

		_, _, err = sk.Undelegate(ctx, oldAddr, valAddr, shares)
		if err != nil {
			return []OldDelegation{}, err
		}

		oldDelegations = append(oldDelegations, OldDelegation{
			Delegation: delegation, DelegationAmount: delegatedAmount,
		})
	}

	// complete all existing unbonding delegations
	undelegations, err := sk.GetAllUnbondingDelegations(ctx, oldAddr)
	if err != nil {
		return oldDelegations, err
	}
	for _, ubd := range undelegations {
		validatorValAddr, _ := sdk.ValAddressFromBech32(ubd.ValidatorAddress)

		blockTime := ctx.BlockTime()
		for i := range ubd.Entries {
			ubd.Entries[i].CompletionTime = blockTime
		}

		if err := sk.SetUnbondingDelegation(ctx, ubd); err != nil {
			return oldDelegations, err
		}
		_, err := sk.CompleteUnbonding(ctx, oldAddr, validatorValAddr)
		if err != nil {
			return oldDelegations, err
		}
	}

	return oldDelegations, nil
}

// Migrate delegations,redelegations and unbonding delegations
func migrateDelegations(ctx sdk.Context, sk *staking.Keeper, newAddr sdk.AccAddress,
	delegations []OldDelegation,
) error {
	// update delegations, unbond and delegate from new address
	for _, delegation := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(delegation.Delegation.GetValidatorAddr())
		if err != nil {
			return err
		}
		validator, err := sk.GetValidator(ctx, valAddr)
		if err != nil {
			return fmt.Errorf("validator not found: %s from delegation %s",
				delegation.Delegation.ValidatorAddress, delegation.Delegation.DelegatorAddress)
		}

		_, err = sk.Delegate(ctx, newAddr, delegation.DelegationAmount, stakingtypes.Unbonded, validator, true)
		if err != nil {
			return err
		}
	}

	return nil
}

// Migrate balances
func migrateBalances(ctx sdk.Context, bk bank.Keeper, oldAddr, newAddr sdk.AccAddress) error {
	spendable := bk.SpendableCoins(ctx, oldAddr)
	if err := bk.SendCoins(ctx, oldAddr, newAddr, spendable); err != nil {
		return err
	}
	return nil
}
