package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
)

const forkHeight = 4426144

// BeginBlockFork updates, the IBC Transfer module's SendEnabled and ReceiveEnabled parameters are set to true.
func BeginBlockFork(ctx sdk.Context, app *PassageApp) {
	if ctx.BlockHeight() == forkHeight {
		ctx.Logger().Info("Applying Passage v2.1 upgrade. Enabling IBC transfers")
		app.TransferKeeper.SetParams(ctx, types.Params{
			SendEnabled:    true,
			ReceiveEnabled: true,
		})
	}
}
