package app

import (
	"cosmossdk.io/core/header"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSimAppExportAndBlockedAddrs(t *testing.T) {
	app := Setup(false, t)
	for acc := range maccPerms {
		require.True(t, app.BankKeeper.BlockedAddr(app.AccountKeeper.GetModuleAddress(acc)), "module account %s must remain blocked", acc)
	}
}

func TestGetMaccPerms(t *testing.T) {
	dup := GetMaccPerms()
	require.Equal(t, maccPerms, dup)
	delete(dup, "mint")
	require.Equal(t, maccPerms, GetMaccPerms())
}

func TestUpgradeStateOnGenesis(t *testing.T) {
	app := Setup(false, t)
	ctx := app.NewContextLegacy(false, tmproto.Header{})
	vm, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.NoError(t, err)
	for name, mod := range app.mm.Modules {
		if cv, ok := mod.(module.HasConsensusVersion); ok {
			require.Equal(t, cv.ConsensusVersion(), vm[name], name)
		}
	}
}

// Exercise the registered security handler through the upgrade keeper, using
// initialized state instead of the obsolete skipped SDK registry harness.
func TestSecurityUpgrade(t *testing.T) {
	app := Setup(false, t)
	ctx := app.NewContextLegacy(false, tmproto.Header{Height: 1})
	require.True(t, app.UpgradeKeeper.HasHandler("v4.1.0"))
	before, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.NoError(t, err)
	wasmBefore := app.WasmKeeper.GetParams(ctx)
	supplyBefore := app.BankKeeper.GetSupply(ctx, "upasg")
	plan := upgradetypes.Plan{Name: "v4.1.0", Height: 2}
	require.NoError(t, app.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx.WithBlockHeight(2).WithHeaderInfo(header.Info{Height: 2}), plan))
	after, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, wasmBefore, app.WasmKeeper.GetParams(ctx))
	require.Equal(t, supplyBefore, app.BankKeeper.GetSupply(ctx, "upasg"))
	height, err := app.UpgradeKeeper.GetDoneHeight(ctx, plan.Name)
	require.NoError(t, err)
	require.Equal(t, int64(2), height)
}
