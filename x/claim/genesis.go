package claim

import (
	"time"

	"github.com/envadiv/Passage3D/x/claim/keeper"
	"github.com/envadiv/Passage3D/x/claim/types"
	abci "github.com/tendermint/tendermint/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the claim module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) []abci.ValidatorUpdate {
	// set up the module account with balance
	k.CreateModuleAccount(ctx, genState.ModuleAccountBalance)

	// If it's the chain genesis, set the airdrop start time to be now, and set up the needed module accounts.
	if genState.Params.AirdropStartTime.Equal(time.Time{}) {
		genState.Params.AirdropStartTime = ctx.BlockTime()
	}
	err := k.SetClaimRecords(ctx, genState.ClaimRecords)
	if err != nil {
		panic(err)
	}

	k.SetParams(ctx, genState.Params)
	return nil
}

// ExportGenesis returns the claim module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	params := k.GetParams(ctx)
	genesis.Params = params
	genesis.ModuleAccountBalance = k.GetModuleAccountBalance(ctx)
	genesis.ClaimRecords = k.GetClaimRecords(ctx)
	return genesis
}
