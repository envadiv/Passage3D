package keeper

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/envadiv/Passage3D/x/claim/types"
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
		actual := existingModuleAcctBalance.Add(amount)
		ctx.Logger().Info(fmt.Sprintf("WARNING! There is a bug in claims on InitGenesis, that you are subject to."+
			" You likely expect the claims module account balance to be %d %s, but it will actually be %d %s due to this bug.",
			amount.Amount.Int64(), amount.Denom, actual.Amount.Int64(), actual.Denom))
	}

	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, mintCoins); err != nil {
		panic(err)
	}
}
