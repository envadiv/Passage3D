package claim_test

import (
	"github.com/envadiv/Passage3D/x/claim"
	"github.com/envadiv/Passage3D/x/claim/types"
	"testing"
	"time"

	simapp "github.com/envadiv/Passage3D/app"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

var now = time.Now().UTC()
var acc1 = sdk.AccAddress("addr1---------------")
var acc2 = sdk.AccAddress("addr2---------------")
var testGenesis = types.GenesisState{
	ModuleAccountBalance: sdk.NewInt64Coin(types.DefaultClaimDenom, 750000000),
	Params: types.Params{
		AirdropEnabled:     true,
		AirdropStartTime:   now,
		DurationUntilDecay: types.DefaultDurationUntilDecay,
		DurationOfDecay:    types.DefaultDurationOfDecay,
		ClaimDenom:         types.DefaultClaimDenom,
	},
	ClaimRecords: []types.ClaimRecord{
		{
			Address:                acc1.String(),
			InitialClaimableAmount: sdk.Coins{sdk.NewInt64Coin(types.DefaultClaimDenom, 1000000000)},
			ActionCompleted:        []bool{true, false, true, true},
		},
		{
			Address:                acc2.String(),
			InitialClaimableAmount: sdk.Coins{sdk.NewInt64Coin(types.DefaultClaimDenom, 500000000)},
			ActionCompleted:        []bool{false, false, false, false},
		},
	},
}

func TestClaimInitGenesis(t *testing.T) {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})
	ctx = ctx.WithBlockTime(now.Add(time.Second))
	genesis := testGenesis
	claim.InitGenesis(ctx, app.ClaimKeeper, genesis)
	coin := app.ClaimKeeper.GetModuleAccountBalance(ctx)
	require.Equal(t, coin.String(), genesis.ModuleAccountBalance.String())

	params := app.ClaimKeeper.GetParams(ctx)
	require.Equal(t, params, genesis.Params)

	claimRecords := app.ClaimKeeper.GetClaimRecords(ctx)
	require.Equal(t, claimRecords, genesis.ClaimRecords)
}

func TestClaimExportGenesis(t *testing.T) {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})
	ctx = ctx.WithBlockTime(now.Add(time.Second))
	genesis := testGenesis
	claim.InitGenesis(ctx, app.ClaimKeeper, genesis)

	claimRecord, err := app.ClaimKeeper.GetClaimRecord(ctx, acc2)
	require.NoError(t, err)
	require.Equal(t, claimRecord, types.ClaimRecord{
		Address:                acc2.String(),
		InitialClaimableAmount: sdk.Coins{sdk.NewInt64Coin(types.DefaultClaimDenom, 500000000)},
		ActionCompleted:        []bool{false, false, false, false},
	})

	claimableAmount, err := app.ClaimKeeper.ClaimCoinsForAction(ctx, acc2, types.ActionDelegateStake)
	require.NoError(t, err)
	require.Equal(t, claimableAmount, sdk.Coins{sdk.NewInt64Coin(types.DefaultClaimDenom, 500000000)})

	genesisExported := claim.ExportGenesis(ctx, app.ClaimKeeper)
	require.Equal(t, genesisExported.ModuleAccountBalance, genesis.ModuleAccountBalance.Sub(claimableAmount[0]))
	require.Equal(t, genesisExported.Params, genesis.Params)
	require.Equal(t, genesisExported.ClaimRecords, []types.ClaimRecord{
		{
			Address:                acc1.String(),
			InitialClaimableAmount: sdk.Coins{sdk.NewInt64Coin(types.DefaultClaimDenom, 1000000000)},
			ActionCompleted:        []bool{true, false, true, true},
		},
		{
			Address:                acc2.String(),
			InitialClaimableAmount: sdk.Coins{sdk.NewInt64Coin(types.DefaultClaimDenom, 500000000)},
			ActionCompleted:        []bool{true, false, false, false},
		},
	})
}

func TestMarshalUnmarshalGenesis(t *testing.T) {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})
	ctx = ctx.WithBlockTime(now.Add(time.Second))

	encodingConfig := simapp.MakeTestEncodingConfig()
	appCodec := encodingConfig.Marshaler
	am := claim.NewAppModule(appCodec, app.ClaimKeeper)

	genesis := testGenesis
	claim.InitGenesis(ctx, app.ClaimKeeper, genesis)

	genesisExported := am.ExportGenesis(ctx, appCodec)
	assert.NotPanics(t, func() {
		app := simapp.Setup(false)
		ctx := app.BaseApp.NewContext(false, tmproto.Header{})
		ctx = ctx.WithBlockTime(now.Add(time.Second))
		am := claim.NewAppModule(appCodec, app.ClaimKeeper)
		am.InitGenesis(ctx, appCodec, genesisExported)
	})
}
