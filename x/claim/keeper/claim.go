package keeper

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"

	"github.com/envadiv/Passage3D/x/claim/types"
	"github.com/gogo/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// GetModuleAccountAddress gets module account address of claim module
func (k Keeper) GetModuleAccountAddress(ctx sdk.Context) sdk.AccAddress {
	return k.accountKeeper.GetModuleAddress(types.ModuleName)
}

// GetModuleAccountBalance gets the airdrop coin balance of module account
func (k Keeper) GetModuleAccountBalance(ctx sdk.Context) sdk.Coin {
	moduleAccAddr := k.GetModuleAccountAddress(ctx)
	return k.bankKeeper.GetBalance(ctx, moduleAccAddr, types.DefaultClaimDenom)
}

// CreateModuleAccount creates the module account with amount
func (k Keeper) CreateModuleAccount(ctx sdk.Context, amount sdk.Coin) {
	moduleAcc := authtypes.NewEmptyModuleAccount(types.ModuleName, authtypes.Minter)
	k.accountKeeper.SetModuleAccount(ctx, moduleAcc)

	mintCoins := sdk.NewCoins(amount)

	existingModuleAcctBalance := k.bankKeeper.GetBalance(ctx, k.accountKeeper.GetModuleAddress(types.ModuleName), amount.Denom)
	if existingModuleAcctBalance.IsPositive() {
		if !existingModuleAcctBalance.Equal(amount) {
			ctx.Logger().Info(fmt.Sprintf("WARNING! There is a bug in claims on InitGenesis, that you are subject to."+
				" You likely expect the claims module account balance to be %s, but it will actually be %s due to this bug.",
				amount.String(), existingModuleAcctBalance.String()))
		}
	} else {
		if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, mintCoins); err != nil {
			panic(err)
		}
	}
}

// SetClaimRecords set claimable amount from balances object
func (k Keeper) SetClaimRecords(ctx sdk.Context, claimRecords []types.ClaimRecord) error {
	for _, claimRecord := range claimRecords {
		err := k.SetClaimRecord(ctx, claimRecord)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetClaimRecord sets a claim record for an address in store
func (k Keeper) SetClaimRecord(ctx sdk.Context, claimRecord types.ClaimRecord) error {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.ClaimRecordsStorePrefix)

	bz, err := proto.Marshal(&claimRecord)
	if err != nil {
		return err
	}

	addr, err := sdk.AccAddressFromBech32(claimRecord.Address)
	if err != nil {
		return err
	}

	prefixStore.Set(addr, bz)
	return nil
}

// UpdateClaimRecord updates a claim record if a entry already exists in the store else adds a new entry.
func (k Keeper) UpdateClaimRecord(ctx sdk.Context, claimRecord types.ClaimRecord) (sdk.Coin, error) {
	addr, err := sdk.AccAddressFromBech32(claimRecord.Address)
	if err != nil {
		return sdk.Coin{}, err
	}

	// if the record found in the state we no need to check the entries present in the claimRecord
	existingClaimRecord, err := k.GetClaimRecord(ctx, addr)
	if err != nil {
		return sdk.Coin{}, err
	}

	var effectiveAmount sdk.Coin
	var int643800 int64 = 3800
	var int649322 int64 = 9322

	if len(existingClaimRecord.ClaimableAmount) != 0 {
		amountInt := existingClaimRecord.ClaimableAmount[0].Amount
		if amountInt.IsZero() {
			newAmount := amountInt.Int64() / int643800 * int649322

			fmt.Println("************ NEW AMOUNT ***************: ", newAmount)

			newCoin := sdk.NewCoin(claimRecord.ClaimableAmount[0].Denom, sdk.NewInt(newAmount))

			effectiveAmount = newCoin.Sub(existingClaimRecord.ClaimableAmount[0])

			fmt.Println("************ Effective Amount ***************: ", effectiveAmount)

			existingClaimRecord.ClaimableAmount = sdk.NewCoins([]sdk.Coin{newCoin}...).Add(claimRecord.ClaimableAmount...)
			claimRecord = existingClaimRecord
		}
	}

	return effectiveAmount, k.SetClaimRecord(ctx, claimRecord)
}

// GetClaimRecords get claimables for genesis export
func (k Keeper) GetClaimRecords(ctx sdk.Context) []types.ClaimRecord {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.ClaimRecordsStorePrefix)

	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	var claimRecords []types.ClaimRecord
	for ; iterator.Valid(); iterator.Next() {

		claimRecord := types.ClaimRecord{}

		err := proto.Unmarshal(iterator.Value(), &claimRecord)
		if err != nil {
			panic(err)
		}

		claimRecords = append(claimRecords, claimRecord)
	}
	return claimRecords
}

// GetClaimRecord returns the claim record for a specific address
func (k Keeper) GetClaimRecord(ctx sdk.Context, addr sdk.AccAddress) (types.ClaimRecord, error) {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.ClaimRecordsStorePrefix)
	if !prefixStore.Has(addr) {
		return types.ClaimRecord{}, nil
	}
	bz := prefixStore.Get(addr)

	claimRecord := types.ClaimRecord{}
	err := proto.Unmarshal(bz, &claimRecord)
	if err != nil {
		return types.ClaimRecord{}, err
	}

	return claimRecord, nil
}

// GetClaimableAmountForAction returns claimable amount for a specific action done by an address
func (k Keeper) GetClaimableAmountForAction(ctx sdk.Context, addr sdk.AccAddress, action int32) (sdk.Coin, error) {
	claimRecord, err := k.GetClaimRecord(ctx, addr)
	if err != nil {
		return sdk.Coin{}, err
	}

	if claimRecord.Address == "" {
		return sdk.Coin{}, nil
	}

	// if action already completed, nothing is claimable
	if claimRecord.ActionCompleted[action] {
		return sdk.Coin{}, nil
	}

	params := k.GetParams(ctx)

	claimablePerAction := claimRecord.ClaimableAmount[action]

	elapsedAirdropTime := ctx.BlockTime().Sub(params.AirdropStartTime)
	// Are we early enough in the airdrop s.t. there's no decay?
	if elapsedAirdropTime <= params.DurationUntilDecay {
		return claimablePerAction, nil
	}

	// The entire airdrop has completed
	if elapsedAirdropTime > params.DurationUntilDecay+params.DurationOfDecay {
		return sdk.Coin{}, nil
	}

	// Positive, since goneTime > params.DurationUntilDecay
	decayTime := elapsedAirdropTime - params.DurationUntilDecay
	decayPercent := sdk.NewDec(decayTime.Nanoseconds()).QuoInt64(params.DurationOfDecay.Nanoseconds())
	claimablePercent := sdk.OneDec().Sub(decayPercent)

	claimablePerAction = sdk.NewCoin(claimablePerAction.Denom, claimablePerAction.Amount.ToDec().Mul(claimablePercent).RoundInt())
	return claimablePerAction, nil
}

// GetUserTotalClaimable returns total claimable amount of an address
func (k Keeper) GetUserTotalClaimable(ctx sdk.Context, addr sdk.AccAddress) (sdk.Coins, error) {
	claimRecord, err := k.GetClaimRecord(ctx, addr)

	if err != nil {
		return sdk.Coins{}, err
	}
	if claimRecord.Address == "" {
		return sdk.Coins{}, nil
	}

	totalClaimable := sdk.Coins{}

	for action := range types.Action_name {
		claimableForAction, err := k.GetClaimableAmountForAction(ctx, addr, action)
		if err != nil {
			return sdk.Coins{}, err
		}
		if !claimableForAction.IsNil() {
			totalClaimable = totalClaimable.Add(claimableForAction)
		}
	}
	return totalClaimable, nil
}

// ClaimCoinsForAction remove claimable amount entry and transfer it to user's account
func (k Keeper) ClaimCoinsForAction(ctx sdk.Context, addr sdk.AccAddress, action int32) (sdk.Coin, error) {
	params := k.GetParams(ctx)
	if !params.IsAirdropEnabled(ctx.BlockTime()) {
		return sdk.Coin{}, types.ErrAirdropNotEnabled
	}

	claimableAmount, err := k.GetClaimableAmountForAction(ctx, addr, action)
	if err != nil {
		return claimableAmount, err
	}

	if claimableAmount.IsNil() {
		return claimableAmount, nil
	}

	claimRecord, err := k.GetClaimRecord(ctx, addr)
	if err != nil {
		return sdk.Coin{}, err
	}

	err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, sdk.NewCoins(claimableAmount))
	if err != nil {
		return sdk.Coin{}, err
	}

	claimRecord.ActionCompleted[action] = true

	err = k.SetClaimRecord(ctx, claimRecord)
	if err != nil {
		return claimableAmount, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeClaim,
			sdk.NewAttribute(sdk.AttributeKeySender, addr.String()),
			sdk.NewAttribute(sdk.AttributeKeyAction, types.Action_name[action]),
			sdk.NewAttribute(sdk.AttributeKeyAmount, claimableAmount.String()),
		),
	})

	return claimableAmount, nil
}

// fundRemainingToCommunity fund remaining to the community when airdrop period end
func (k Keeper) fundRemainingToCommunity(ctx sdk.Context) error {
	moduleAccAddr := k.GetModuleAccountAddress(ctx)
	amt := k.GetModuleAccountBalance(ctx)
	ctx.Logger().Info(fmt.Sprintf(
		"Sending %d %s to community pool, corresponding to the 'unclaimed airdrop'", amt.Amount.Int64(), amt.Denom))
	return k.distrKeeper.FundCommunityPool(ctx, sdk.NewCoins(amt), moduleAccAddr)
}

func (k Keeper) EndAirdrop(ctx sdk.Context) error {
	err := k.fundRemainingToCommunity(ctx)
	if err != nil {
		return err
	}
	k.clearInitialClaimables(ctx)
	return nil
}

// ClearClaimables clear claimable amounts
func (k Keeper) clearInitialClaimables(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, types.ClaimRecordsStorePrefix)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		store.Delete(key)
	}
}
