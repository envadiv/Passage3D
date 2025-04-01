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

		if err := migrateAccount(ctx, appCodec, ak, oldAccount, newAddr); err != nil {
			return fmt.Errorf("failed to migrate account: %w", err)
		}

		// unbond old delegation
		delegations, err := unbondOldDelegations(ctx, bk, sk, oldAddr)
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

		if err := migrateAuthorizations(ctx, azk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate authorizations: %w", err)
		}

		if err := migrateFeeGrants(ctx, fk, oldAddr, newAddr); err != nil {
			return fmt.Errorf("failed to migrate feegrants: %w", err)
		}

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
	oldAcc.StartTime = ctx.BlockTime().Unix()
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

func unbondOldDelegations(ctx sdk.Context, bk bank.Keeper, sk staking.Keeper,
	oldAddr sdk.AccAddress,
) ([]OldDelegation, error) {
	// complete all existing redelegations
	sk.IterateDelegatorRedelegations(ctx, oldAddr, func(red stakingtypes.Redelegation) (stop bool) {
		// set all entry completionTime to now so we can complete re-delegation
		redelegationSrc, _ := sdk.ValAddressFromBech32(red.ValidatorSrcAddress)
		redelegationDst, _ := sdk.ValAddressFromBech32(red.ValidatorDstAddress)

		blockTime := ctx.BlockTime()
		for i := range red.Entries {
			red.Entries[i].CompletionTime = blockTime
		}
		sk.SetRedelegation(ctx, red)
		_, err := sk.CompleteRedelegation(ctx, oldAddr, redelegationSrc, redelegationDst)
		if err != nil {
			panic(err)
		}

		return false
	})

	delegations := sk.GetAllDelegatorDelegations(ctx, oldAddr)
	oldDelegations := []OldDelegation{}
	for _, delegation := range delegations {
		valAddr := delegation.GetValidatorAddr()
		shares := delegation.GetShares()

		validator, found := sk.GetValidator(ctx, valAddr)
		if !found {
			return oldDelegations, fmt.Errorf("validator not found: %s from delegation %s",
				delegation.ValidatorAddress, delegation.DelegatorAddress)
		}

		delegatedAmount := validator.TokensFromShares(shares).TruncateInt()

		_, err := sk.Undelegate(ctx, oldAddr, valAddr, shares)
		if err != nil {
			return []OldDelegation{}, err
		}

		oldDelegations = append(oldDelegations, OldDelegation{
			Delegation: delegation, DelegationAmount: delegatedAmount,
		})
	}

	// complete all existing unbonding delegations
	undelegations := sk.GetAllUnbondingDelegations(ctx, oldAddr)
	for _, ubd := range undelegations {
		validatorValAddr, _ := sdk.ValAddressFromBech32(ubd.ValidatorAddress)

		blockTime := ctx.BlockTime()
		for i := range ubd.Entries {
			ubd.Entries[i].CompletionTime = blockTime
		}

		sk.SetUnbondingDelegation(ctx, ubd)
		_, err := sk.CompleteUnbonding(ctx, oldAddr, validatorValAddr)
		if err != nil {
			return oldDelegations, err
		}
	}

	return oldDelegations, nil
}

// Migrate delegations,redelegations and unbonding delegations
func migrateDelegations(ctx sdk.Context, sk staking.Keeper, newAddr sdk.AccAddress,
	delegations []OldDelegation,
) error {
	// update delegations, unbond and delegate from new address
	for _, delegation := range delegations {
		validator, found := sk.GetValidator(ctx, delegation.Delegation.GetValidatorAddr())
		if !found {
			return fmt.Errorf("validator not found: %s from delegation %s",
				delegation.Delegation.ValidatorAddress, delegation.Delegation.DelegatorAddress)
		}

		_, err := sk.Delegate(ctx, newAddr, delegation.DelegationAmount, stakingtypes.Unbonded, validator, true)
		if err != nil {
			return err
		}
	}

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
