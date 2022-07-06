package claim

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/envadiv/Passage3D/x/claim/keeper"
	abci "github.com/tendermint/tendermint/abci/types"
)

// BeginBlocker is called on every block
func BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock, k keeper.Keeper) {
}

// EndBlocker is called on every block
func EndBlocker(ctx sdk.Context, k keeper.Keeper) {
	params := k.GetParams(ctx)
	if !params.IsAirdropEnabled(ctx.BlockTime()) {
		return
	}
	// End Airdrop
	goneTime := ctx.BlockTime().Sub(params.AirdropStartTime)
	if goneTime > params.DurationUntilDecay+params.DurationOfDecay {
		// airdrop time passed
		err := k.EndAirdrop(ctx)
		if err != nil {
			panic(err)
		}
	}
}
