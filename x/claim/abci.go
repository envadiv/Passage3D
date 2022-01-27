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
func EndBlocker(ctx sdk.Context, k keeper.Keeper) []abci.ValidatorUpdate {
	return []abci.ValidatorUpdate{}
}
