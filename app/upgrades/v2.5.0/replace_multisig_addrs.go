package v2_5

import (
	"context"
	"fmt"
	"os"

	sdk "github.com/cosmos/cosmos-sdk/types"
	pageQuery "github.com/cosmos/cosmos-sdk/types/query"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
	"github.com/gogo/protobuf/codec"
)

type AddressMigration struct {
	OldAddress string             `json:"old_address"`
	NewAccount authtypes.AccountI `json:"new_account"`
}

func MigrateMultisigAddresses(
	ctx sdk.Context,
	appCodec codec.Codec,
	dk distribution.Keeper,
	bk bank.Keeper,
	ak auth.AccountKeeper,
	sk staking.Keeper,
	azk authzkeeper.Keeper,
	ck claim.Keeper,
) error {
	migrations, err := LoadAddressMigrations(appCodec, "migrations.json")
	if err != nil {
		return err
	}

	for _, m := range migrations {
		oldAddr, err := sdk.AccAddressFromBech32(m.OldAddress)
		if err != nil {
			return fmt.Errorf("invalid bech32 old address: %s, error: %w", m.OldAddress, err)
		}

		oldAccount := ak.GetAccount(ctx, oldAddr)
		if oldAccount == nil {
			return fmt.Errorf("old account %s not found", m.OldAddress)
		}

		newAccount := m.NewAccount
		newAddr := newAccount.GetAddress()

		if err := migrateAccount(ctx, ak, oldAccount, newAccount); err != nil {
			return fmt.Errorf("failed to migrate account: %w", err)
		}

		if err := migrateDelegations(ctx, sk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate delegations: %w", err)
		}

		if err := migrateAuthorizations(ctx, azk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate authorizations: %w", err)
		}

		// send spendable balance from old account to new account
		if err := migrateBalances(ctx, bk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate balances: %w", err)
		}
	}

	return nil
}

func LoadAddressMigrations(appCodec codec.Codec, filePath string) ([]AddressMigration, error) {
	// Read the JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration file: %w", err)
	}

	// Unmarshal into slice of AddressMigration
	var migrations []AddressMigration
	if err := appCodec.Unmarshal(data, &migrations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal migrations: %w", err)
	}

	return migrations, nil
}

// Migrate account information based on type
func migrateAccount(ctx sdk.Context, ak auth.AccountKeeper, oldAccount, newAccount authtypes.AccountI) error {
	switch oldAcc := oldAccount.(type) {
	case *vestingtypes.PeriodicVestingAccount:
		return migrateVestingAccount(ctx, ak, oldAcc, newAccount)

	case *authtypes.BaseAccount:
		return migrateBaseAccount(ctx, ak, newAccount)

	default:
		return fmt.Errorf("not supported account type for upgrade")
	}
}

// Migrate vesting account
func migrateVestingAccount(ctx sdk.Context, ak auth.AccountKeeper, oldAcc *vestingtypes.PeriodicVestingAccount, newAccount authtypes.AccountI) error {
	vestingPeriods := oldAcc.VestingPeriods

	// Unlock old vesting periods
	for i := range oldAcc.VestingPeriods {
		oldAcc.VestingPeriods[i].Length = 0
	}
	ak.SetAccount(ctx, oldAcc)

	// Create new vesting account with old vesting periods
	newVestingAccount := vestingtypes.NewPeriodicVestingAccount(
		authtypes.NewBaseAccount(newAccount.GetAddress(), newAccount.GetPubKey(),
			newAccount.GetAccountNumber(), newAccount.GetSequence()),
		oldAcc.OriginalVesting, oldAcc.StartTime, vestingPeriods,
	)
	ak.SetAccount(ctx, newVestingAccount)

	return nil
}

// Migrate base account
func migrateBaseAccount(ctx sdk.Context, ak auth.AccountKeeper, newAccount authtypes.AccountI) error {
	newBaseAcc := authtypes.NewBaseAccount(newAccount.GetAddress(), newAccount.GetPubKey(),
		newAccount.GetAccountNumber(), newAccount.GetSequence())
	ak.SetAccount(ctx, newBaseAcc)
	return nil
}

// Migrate delegations,redelegations and unbonding delegations
func migrateDelegations(ctx sdk.Context, sk staking.Keeper, oldAddr, newAddr sdk.AccAddress) error {
	// update delegations, unbond and delegate from new address
	delegations := sk.GetAllDelegatorDelegations(ctx, oldAddr)
	for _, delegation := range delegations {
		validator, found := sk.GetValidator(ctx, delegation.GetValidatorAddr())
		if !found {
			return fmt.Errorf("validator not found: %s from delegation %s", delegation.ValidatorAddress, delegation.DelegatorAddress)
		}

		amount, err := sk.Unbond(ctx, oldAddr, delegation.GetValidatorAddr(), delegation.GetShares())
		if err != nil {
			return err
		}

		_, err = sk.Delegate(ctx, newAddr, amount, validator.GetStatus(), validator, false)
		if err != nil {
			return err
		}
	}

	// update existing unbonding delegations
	sk.IterateDelegatorUnbondingDelegations(ctx, oldAddr, func(ubd stakingtypes.UnbondingDelegation) (stop bool) {
		sk.RemoveUnbondingDelegation(ctx, ubd)
		ubd.DelegatorAddress = newAddr.String()
		sk.SetUnbondingDelegation(ctx, ubd)
		for _, entry := range ubd.Entries {
			sk.InsertUBDQueue(ctx, ubd, entry.CompletionTime)
		}
		return false
	})

	// update existing redelegations
	sk.IterateDelegatorRedelegations(ctx, oldAddr, func(red stakingtypes.Redelegation) (stop bool) {
		sk.RemoveRedelegation(ctx, red)
		red.DelegatorAddress = newAddr.String()
		sk.SetRedelegation(ctx, red)
		for _, entry := range red.Entries {
			sk.InsertRedelegationQueue(ctx, red, entry.CompletionTime)
		}
		return false
	})

	return nil
}

// Migrate authorizations
func migrateAuthorizations(ctx sdk.Context, azk authzkeeper.Keeper, oldAddress, newAddress sdk.AccAddress) error {
	var allGrants []*authz.GrantAuthorization
	var nextKey []byte

	for {
		resp, err := azk.GranterGrants(context.Background(), &authz.QueryGranterGrantsRequest{
			Granter: oldAddress.String(),
			Pagination: &pageQuery.PageRequest{
				Limit: 100,
				Key:   nextKey,
			},
		})
		if err != nil {
			return err
		}

		allGrants = append(allGrants, resp.Grants...)
		nextKey = resp.Pagination.NextKey

		// If nextKey is nil, we've retrieved all pages
		if nextKey == nil {
			break
		}
	}

	// Process all grants
	for _, grant := range allGrants {
		granteeAddr, err := sdk.AccAddressFromBech32(grant.Grantee)
		if err != nil {
			return fmt.Errorf("invalid bech32 grantee address: %s, error: %w", grant.Grantee, err)
		}

		auth, ok := grant.Authorization.GetCachedValue().(authz.Authorization)
		if !ok {
			return fmt.Errorf("invalid authorization for granter: %s, grantee: %s", grant.Granter, grant.Grantee)
		}

		// Delete old grant
		if err := azk.DeleteGrant(ctx, granteeAddr, oldAddress, auth.MsgTypeURL()); err != nil {
			return err
		}

		// Create new grant with the updated address
		if err := azk.SaveGrant(ctx, granteeAddr, newAddress, auth, grant.Expiration); err != nil {
			return err
		}
	}

	return nil
}

// Migrate balances
func migrateBalances(ctx sdk.Context, bk bank.Keeper, oldAddr, newAddr sdk.AccAddress) error {
	spendable := bk.SpendableCoins(ctx, oldAddr)
	return bk.SendCoins(ctx, oldAddr, newAddr, spendable)
}
