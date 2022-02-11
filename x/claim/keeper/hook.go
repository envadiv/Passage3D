package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/envadiv/Passage3D/x/claim/types"
)

func (k Keeper) AfterDelegationModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	_, err := k.ClaimCoinsForAction(ctx, delAddr, types.ActionDelegateStake)
	if err != nil {
		panic(err.Error())
	}
}
