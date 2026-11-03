package claim_test

import (
	"testing"
	"time"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/x/claim"
	"github.com/envadiv/Passage3D/x/claim/types"
)

var (
	now  = time.Now().UTC()
	acc1 = sdk.AccAddress("addr1---------------")
	acc2 = sdk.AccAddress("addr2---------------")
)

// testGenesis returns a new claim genesis state with an active airdrop that
// started at now and two claim records.
func testGenesis() types.GenesisState {
	return types.GenesisState{
		ModuleAccountBalance: sdk.NewInt64Coin(types.DefaultClaimDenom, 1500000000),
		Params: types.Params{
			AirdropEnabled:     true,
			AirdropStartTime:   now,
			DurationUntilDecay: types.DefaultDurationUntilDecay,
			DurationOfDecay:    types.DefaultDurationOfDecay,
			ClaimDenom:         types.DefaultClaimDenom,
		},
		ClaimRecords: []types.ClaimRecord{
			{
				Address:         acc1.String(),
				ClaimableAmount: []sdk.Coin{sdk.NewInt64Coin(types.DefaultClaimDenom, 1000000000)},
				ActionCompleted: []bool{false},
			},
			{
				Address:         acc2.String(),
				ClaimableAmount: []sdk.Coin{sdk.NewInt64Coin(types.DefaultClaimDenom, 500000000)},
				ActionCompleted: []bool{false},
			},
		},
	}
}

// deliverCtx returns a non-CheckTx context for the next block that writes
// straight to the app's commit multistore, so the next Commit persists it.
// app.Setup has already committed its first block, so there is no open
// FinalizeBlock state to branch from.
func deliverCtx(a *app.PassageApp, blockTime time.Time) sdk.Context {
	return a.BaseApp.NewUncachedContext(false, cmtproto.Header{Height: a.LastBlockHeight() + 1, Time: blockTime})
}

// requireUniqueAccountNumbers fails if two accounts share an account number.
func requireUniqueAccountNumbers(t *testing.T, ctx sdk.Context, a *app.PassageApp) {
	t.Helper()
	owners := make(map[uint64]string)
	a.AccountKeeper.IterateAccounts(ctx, func(acc sdk.AccountI) bool {
		num := acc.GetAccountNumber()
		require.NotContains(t, owners, num, "account number %d used by %s and %s", num, owners[num], acc.GetAddress())
		owners[num] = acc.GetAddress().String()
		return false
	})
}

func TestClaimInitGenesis(t *testing.T) {
	a := app.Setup(false, t)
	ctx := deliverCtx(a, now.Add(time.Second))
	genesis := testGenesis()
	claim.InitGenesis(ctx, a.ClaimKeeper, genesis)

	require.Equal(t, genesis.ModuleAccountBalance.String(), a.ClaimKeeper.GetModuleAccountBalance(ctx).String())
	require.Equal(t, genesis.Params, a.ClaimKeeper.GetParams(ctx))
	require.Equal(t, genesis.ClaimRecords, a.ClaimKeeper.GetClaimRecords(ctx))
}

// Regression test: in SDK v0.50 account numbers are a unique index. The claim
// module account used to be created with account number 0, which collided with
// the first genesis account and made InitGenesis panic with
// "collections: conflict: index uniqueness constrain violation: 0".
func TestClaimInitGenesisWithGenesisAccountNumberZero(t *testing.T) {
	a := app.NewTestApp(0)
	ctx := a.BaseApp.NewUncachedContext(false, cmtproto.Header{Height: 1, Time: now})

	authGenesis := authtypes.NewGenesisState(authtypes.DefaultParams(),
		authtypes.GenesisAccounts{authtypes.NewBaseAccount(acc1, nil, 0, 0)})
	a.AccountKeeper.InitGenesis(ctx, *authGenesis)
	require.Equal(t, uint64(0), a.AccountKeeper.GetAccount(ctx, acc1).GetAccountNumber())

	genesis := testGenesis()
	require.NotPanics(t, func() { claim.InitGenesis(ctx, a.ClaimKeeper, genesis) })

	moduleAcc := a.AccountKeeper.GetAccount(ctx, a.ClaimKeeper.GetModuleAccountAddress(ctx))
	require.NotNil(t, moduleAcc)
	_, isModuleAcc := moduleAcc.(sdk.ModuleAccountI)
	require.True(t, isModuleAcc)
	require.NotEqual(t, uint64(0), moduleAcc.GetAccountNumber())
	requireUniqueAccountNumbers(t, ctx, a)

	require.Equal(t, genesis.ModuleAccountBalance.String(), a.ClaimKeeper.GetModuleAccountBalance(ctx).String())
	require.Equal(t, genesis.ClaimRecords, a.ClaimKeeper.GetClaimRecords(ctx))

	// Running InitGenesis again (e.g. a re-import) must reuse the account.
	require.NotPanics(t, func() { claim.InitGenesis(ctx, a.ClaimKeeper, genesis) })
	require.Equal(t, moduleAcc.GetAccountNumber(),
		a.AccountKeeper.GetAccount(ctx, a.ClaimKeeper.GetModuleAccountAddress(ctx)).GetAccountNumber())
	requireUniqueAccountNumbers(t, ctx, a)
}

// The same scenario through a full InitChain: app.Setup's genesis has an
// account with account number 0 and the claim module account is not part of
// the auth genesis, so x/claim InitGenesis has to create it.
func TestInitChainCreatesClaimModuleAccountWithUniqueNumber(t *testing.T) {
	var a *app.PassageApp
	require.NotPanics(t, func() { a = app.Setup(false, t) })

	ctx := a.BaseApp.NewContext(true)
	moduleAcc := a.AccountKeeper.GetAccount(ctx, a.ClaimKeeper.GetModuleAccountAddress(ctx))
	require.NotNil(t, moduleAcc)
	require.NotEqual(t, uint64(0), moduleAcc.GetAccountNumber())
	requireUniqueAccountNumbers(t, ctx, a)
}

func TestClaimExportGenesis(t *testing.T) {
	a := app.Setup(false, t)
	ctx := deliverCtx(a, now.Add(time.Second))
	genesis := testGenesis()
	claim.InitGenesis(ctx, a.ClaimKeeper, genesis)

	claimRecord, err := a.ClaimKeeper.GetClaimRecord(ctx, acc2)
	require.NoError(t, err)
	require.Equal(t, genesis.ClaimRecords[1], claimRecord)

	claimableAmount, err := a.ClaimKeeper.ClaimCoinsForAction(ctx, acc2, types.ActionInitialClaim)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt64Coin(types.DefaultClaimDenom, 500000000), claimableAmount)

	genesisExported := claim.ExportGenesis(ctx, a.ClaimKeeper)
	require.Equal(t, genesis.ModuleAccountBalance.Sub(claimableAmount), genesisExported.ModuleAccountBalance)
	require.Equal(t, genesis.Params, genesisExported.Params)
	require.Equal(t, []types.ClaimRecord{
		genesis.ClaimRecords[0],
		{
			Address:         acc2.String(),
			ClaimableAmount: []sdk.Coin{sdk.NewInt64Coin(types.DefaultClaimDenom, 500000000)},
			ActionCompleted: []bool{true},
		},
	}, genesisExported.ClaimRecords)
}

// TestGenesisExportImportRoundTrip exports the claim state of one app as JSON,
// imports it into a fresh app and checks that re-exporting yields the same JSON.
func TestGenesisExportImportRoundTrip(t *testing.T) {
	encCfg := app.MakeEncodingConfig()
	cdc := encCfg.Marshaler

	src := app.Setup(false, t)
	srcCtx := deliverCtx(src, now.Add(time.Second))
	claim.InitGenesis(srcCtx, src.ClaimKeeper, testGenesis())
	_, err := src.ClaimKeeper.ClaimCoinsForAction(srcCtx, acc1, types.ActionInitialClaim)
	require.NoError(t, err)

	exported := claim.NewAppModule(cdc, src.ClaimKeeper).ExportGenesis(srcCtx, cdc)

	dst := app.Setup(false, t)
	dstCtx := deliverCtx(dst, now.Add(time.Second))
	dstModule := claim.NewAppModule(cdc, dst.ClaimKeeper)
	require.NotPanics(t, func() { dstModule.InitGenesis(dstCtx, cdc, exported) })
	requireUniqueAccountNumbers(t, dstCtx, dst)

	require.JSONEq(t, string(exported), string(dstModule.ExportGenesis(dstCtx, cdc)))
	require.Equal(t, src.ClaimKeeper.GetModuleAccountBalance(srcCtx), dst.ClaimKeeper.GetModuleAccountBalance(dstCtx))
}
