package v2_5

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	pageQuery "github.com/cosmos/cosmos-sdk/types/query"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	feegrant "github.com/cosmos/cosmos-sdk/x/feegrant"
	feegrantkeeper "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	gov "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
)

type AddressMigration struct {
	OldAddress string `json:"old_address"`
	NewAddress string `json:"new_address"`
}

type AddressMap map[string]string

type OldDelegation struct {
	Delegation       stakingtypes.Delegation
	DelegationAmount sdk.Int
}

func MigrateMultisigAddresses(
	ctx sdk.Context,
	appCodec codec.Codec,
	migrations []AddressMigration,
	bk bank.Keeper,
	ak auth.AccountKeeper,
	sk staking.Keeper,
	gk gov.Keeper,
	azk authzkeeper.Keeper,
	fk feegrantkeeper.Keeper,
	ck claim.Keeper,
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

		fmt.Println("Initial Spendable balance......", bk.SpendableCoins(ctx, oldAddr))

		fmt.Println("Initial Accounts.......", ak.GetAccount(ctx, oldAddr))

		if err := migrateAccount(ctx, appCodec, ak, oldAccount, newAddr); err != nil {
			return fmt.Errorf("failed to migrate account: %w", err)
		}

		fmt.Println("Accounts.......", ak.GetAccount(ctx, oldAddr), "New...", ak.GetAccount(ctx, newAddr))

		// unbond old delegation
		delegations, err := unbondOldDelegations(ctx, bk, sk, oldAddr)
		if err != nil {
			return fmt.Errorf("failed to unbond old delegations: %w", err)
		}

		// send spendable balance from old account to new account
		if err := migrateBalances(ctx, bk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate balances: %w", err)
		}

		if err := migrateDelegations(ctx, bk, sk, oldAddr, newAddr, delegations); err != nil {
			return fmt.Errorf("failed to migrate delegations: %w", err)
		}

		fmt.Println("Accounts Last.......", ak.GetAccount(ctx, oldAddr), "New...", ak.GetAccount(ctx, newAddr))

		if err := migrateAuthorizations(ctx, azk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate authorizations: %w", err)
		}

		if err := migrateFeeGrants(ctx, fk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate feegrants: %w", err)
		}

		// send again spendable balance from old account to new account to avoid missing balances
		if err := migrateBalances(ctx, bk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate balances: %w", err)
		}
	}

	// migrate gov votes
	migrateGovVotes(ctx, gk, addressMap)

	return nil
}

// Migrate account information based on type
func migrateAccount(ctx sdk.Context, appCodec codec.Codec, ak auth.AccountKeeper, oldAccount authtypes.AccountI,
	newAddr sdk.AccAddress,
) error {
	switch oldAcc := oldAccount.(type) {
	case *vestingtypes.PeriodicVestingAccount:
		return migrateVestingAccount(ctx, appCodec, ak, oldAcc, newAddr)

	case *authtypes.BaseAccount:
		return migrateBaseAccount(ctx, ak, newAddr)

	default:
		return fmt.Errorf("not supported account type for upgrade")
	}
}

// Migrate vesting account
func migrateVestingAccount(ctx sdk.Context, appCodec codec.Codec, ak auth.AccountKeeper, oldAcc *vestingtypes.PeriodicVestingAccount,
	newAddr sdk.AccAddress,
) error {
	// copy old account to new vesting account
	accBytes, err := appCodec.Marshal(oldAcc)
	if err != nil {
		return err
	}

	var newVestingAccount vestingtypes.PeriodicVestingAccount
	if err := appCodec.Unmarshal(accBytes, &newVestingAccount); err != nil {
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
	ak.SetAccount(ctx, oldAcc)

	return nil
}

// Migrate base account
func migrateBaseAccount(ctx sdk.Context, ak auth.AccountKeeper, newAddr sdk.AccAddress) error {
	newBaseAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	ak.SetAccount(ctx, newBaseAcc)
	return nil
}

func unbondOldDelegations(ctx sdk.Context, bk bank.Keeper, sk staking.Keeper,
	oldAddr sdk.AccAddress,
) ([]OldDelegation, error) {
	delegations := sk.GetAllDelegatorDelegations(ctx, oldAddr)
	oldDelegations := []OldDelegation{}
	bondDenom := sk.GetParams(ctx).BondDenom
	for i, delegation := range delegations {
		fmt.Println("Balance before......", bk.SpendableCoins(ctx, oldAddr))
		fmt.Println("Old Delegations...........", i, delegation)

		amount, err := sk.Unbond(ctx, oldAddr, delegation.GetValidatorAddr(), delegation.GetShares())
		if err != nil {
			return []OldDelegation{}, err
		}

		if err := bk.UndelegateCoinsFromModuleToAccount(ctx, stakingtypes.BondedPoolName, oldAddr,
			sdk.NewCoins(sdk.NewCoin(bondDenom, amount))); err != nil {
			return []OldDelegation{}, err
		}

		oldDelegations = append(oldDelegations, OldDelegation{
			Delegation: delegation, DelegationAmount: amount,
		})

		fmt.Println("Balance After Old......", amount, bk.SpendableCoins(ctx, oldAddr))
	}

	return oldDelegations, nil
}

// Migrate delegations,redelegations and unbonding delegations
func migrateDelegations(ctx sdk.Context, bk bank.Keeper, sk staking.Keeper, oldAddr, newAddr sdk.AccAddress,
	delegations []OldDelegation,
) error {
	// bondDenom := sk.GetParams(ctx).BondDenom
	// update delegations, unbond and delegate from new address
	for i, delegation := range delegations {
		fmt.Println("New Balance before......", bk.SpendableCoins(ctx, newAddr))
		fmt.Println("Delegations...........", i, delegation)
		validator, found := sk.GetValidator(ctx, delegation.Delegation.GetValidatorAddr())
		if !found {
			return fmt.Errorf("validator not found: %s from delegation %s",
				delegation.Delegation.ValidatorAddress, delegation.Delegation.DelegatorAddress)
		}

		_, err := sk.Delegate(ctx, newAddr, delegation.DelegationAmount, stakingtypes.Unbonded, validator, true)
		if err != nil {
			return err
		}

		fmt.Println("Balance After......", bk.SpendableCoins(ctx, newAddr))
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
		goCtx := sdk.WrapSDKContext(ctx)
		resp, err := azk.GranterGrants(goCtx, &authz.QueryGranterGrantsRequest{
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

// Migrate fee grants
func migrateFeeGrants(ctx sdk.Context, fk feegrantkeeper.Keeper, oldAddress, newAddress sdk.AccAddress) error {
	var allGrants []*feegrant.Grant
	var nextKey []byte

	for {
		goCtx := sdk.WrapSDKContext(ctx)
		resp, err := fk.AllowancesByGranter(goCtx, &feegrant.QueryAllowancesByGranterRequest{
			Granter: oldAddress.String(),
			Pagination: &pageQuery.PageRequest{
				Limit: 100,
				Key:   nextKey,
			},
		})
		if err != nil {
			return err
		}

		allGrants = append(allGrants, resp.Allowances...)
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

		g, ok := grant.Allowance.GetCachedValue().(feegrant.FeeAllowanceI)
		if !ok {
			return fmt.Errorf("invalid fee grant for granter: %s, grantee: %s", grant.Granter, grant.Grantee)
		}

		// Create new grant with the updated address
		if err := fk.GrantAllowance(ctx, newAddress, granteeAddr, g); err != nil {
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

// Migrate gov votes
func migrateGovVotes(ctx sdk.Context, gk gov.Keeper, addressMap AddressMap) {
	votes := gk.GetAllVotes(ctx)

	for _, vote := range votes {
		newAddr, found := addressMap[vote.Voter]
		if found {
			vote.Voter = newAddr
			gk.SetVote(ctx, vote)
		}
	}
}
