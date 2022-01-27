package claim

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/envadiv/Passage3D/x/claim/keeper"
	"github.com/envadiv/Passage3D/x/claim/types"
	"time"
)

// InitGenesis initializes the claim module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	// If it's the chain genesis, set the airdrop start time to be now, and set up the needed module accounts.
	if genState.Params.AirdropStartTime.Equal(time.Time{}) {
		genState.Params.AirdropStartTime = ctx.BlockTime()
		k.CreateModuleAccount(ctx, genState.ModuleAccountBalance)
	}

	if err := k.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}
}

// ExportGenesis returns the claim module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(err)
	}
	genesis := types.DefaultGenesis()
	genesis.Params = params
	return genesis
}
