package app

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/core/header"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	paramproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"

	v410 "github.com/envadiv/Passage3D/app/upgrades/v4.1.0"
	claimtypes "github.com/envadiv/Passage3D/x/claim/types"
)

func TestBlockedAddrs(t *testing.T) {
	app := NewTestApp(0)
	for acc := range maccPerms {
		require.True(
			t,
			app.BankKeeper.BlockedAddr(app.AccountKeeper.GetModuleAddress(acc)),
			"module account %s must be a blocked address in the bank keeper", acc,
		)
	}
}

func TestGetMaccPerms(t *testing.T) {
	dup := GetMaccPerms()
	require.Equal(t, maccPerms, dup, "duplicated module account permissions differed from actual module account permissions")
}

// nextBlock finalizes and commits one empty block.
func nextBlock(t *testing.T, app *PassageApp) {
	t.Helper()
	_, err := app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: app.LastBlockHeight() + 1,
		Time:   time.Now().UTC(),
	})
	require.NoError(t, err)
	_, err = app.Commit()
	require.NoError(t, err)
}

// deliverCtx returns a DeliverTx-mode context writing directly to the committed
// multistore (no block in progress is needed).
func deliverCtx(app *PassageApp) sdk.Context {
	return app.NewUncachedContext(false, cmtproto.Header{Height: app.LastBlockHeight() + 1, Time: time.Now().UTC()})
}

// requireUniqueAccountNumbers checks that every account has its own number and
// owns the matching entry of the x/auth account-number index.
func requireUniqueAccountNumbers(t *testing.T, app *PassageApp, ctx sdk.Context) {
	t.Helper()
	seen := map[uint64]string{}
	err := app.AccountKeeper.Accounts.Walk(ctx, nil, func(addr sdk.AccAddress, acc sdk.AccountI) (bool, error) {
		n := acc.GetAccountNumber()
		prev, dup := seen[n]
		require.Falsef(t, dup, "account number %d used by %s and %s", n, prev, addr)
		seen[n] = addr.String()

		owner, err := app.AccountKeeper.Accounts.Indexes.Number.MatchExact(ctx, n)
		require.NoErrorf(t, err, "account number %d of %s is not indexed", n, addr)
		require.Truef(t, owner.Equals(addr), "account number %d indexed to %s instead of %s", n, owner, addr)
		return false, nil
	})
	require.NoError(t, err)
}

// A fresh genesis with a genesis account numbered 0 used to panic in x/claim
// InitGenesis ("collections: conflict: index uniqueness constrain violation: 0").
func TestFreshGenesisHasUniqueAccountNumbers(t *testing.T) {
	app := Setup(false, t)
	ctx := deliverCtx(app)

	claimAcc := app.AccountKeeper.GetAccount(ctx, app.AccountKeeper.GetModuleAddress(claimtypes.ModuleName))
	require.NotNil(t, claimAcc, "claim module account must exist after genesis")
	requireUniqueAccountNumbers(t, app, ctx)
}

// Export (normal and for zero height) must produce a genesis that a new chain can
// import. Zero-height export used to panic ("not yet ported to SDK v0.50").
func TestExportAndReimport(t *testing.T) {
	for _, forZeroHeight := range []bool{false, true} {
		t.Run(fmt.Sprintf("forZeroHeight=%v", forZeroHeight), func(t *testing.T) {
			app := Setup(false, t)
			for i := 0; i < 3; i++ {
				nextBlock(t, app)
			}

			exported, err := app.ExportAppStateAndValidators(forZeroHeight, nil, nil)
			require.NoError(t, err)
			require.NotEmpty(t, exported.Validators)
			if forZeroHeight {
				require.Equal(t, int64(0), exported.Height)
			} else {
				require.Equal(t, app.LastBlockHeight()+1, exported.Height)
			}

			var genState GenesisState
			require.NoError(t, json.Unmarshal(exported.AppState, &genState))

			newApp := NewTestApp(0)
			ctx := newApp.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight()})
			require.NotPanics(t, func() {
				_, err = newApp.mm.InitGenesis(ctx, newApp.AppCodec(), genState)
			})
			require.NoError(t, err)

			oldCtx := deliverCtx(app)
			require.Equal(t,
				app.BankKeeper.GetSupply(oldCtx, BondDenom),
				newApp.BankKeeper.GetSupply(ctx, BondDenom),
				"total supply must survive export/import",
			)
			requireUniqueAccountNumbers(t, newApp, ctx)
		})
	}
}

// Only the requested modules are exported.
func TestExportModulesToExport(t *testing.T) {
	app := Setup(false, t)
	exported, err := app.ExportAppStateAndValidators(false, nil, []string{"bank"})
	require.NoError(t, err)

	var genState GenesisState
	require.NoError(t, json.Unmarshal(exported.AppState, &genState))
	require.Len(t, genState, 1)
	require.Contains(t, genState, "bank")
}

// passage-2 holds two accounts with account number 0: a vesting multisig (owner
// of the index entry) and the claim module account, created under SDK v0.45 by
// the old x/claim InitGenesis. The v4.1.0 upgrade handler must give the claim
// account a fresh number and keep the owner's index entry.
func TestUpgradeV410RepairsDuplicateAccountNumber(t *testing.T) {
	app := Setup(false, t)
	ctx := deliverCtx(app)
	ak := app.AccountKeeper

	// The account that owns number 0 (the vesting multisig on passage-2).
	owner := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	ownerAcc := ak.NewAccountWithAddress(ctx, owner)
	require.NoError(t, ownerAcc.SetAccountNumber(0))
	ak.SetAccount(ctx, ownerAcc)
	indexed, err := ak.Accounts.Indexes.Number.MatchExact(ctx, 0)
	require.NoError(t, err)
	require.True(t, indexed.Equals(owner))

	claimAddr := ak.GetModuleAddress(claimtypes.ModuleName)
	claimAcc := ak.GetAccount(ctx, claimAddr)
	require.NotNil(t, claimAcc)
	require.NotEqual(t, uint64(0), claimAcc.GetAccountNumber())

	// Recreate the passage-2 state: the claim account record says number 0 and the
	// account-number index has no entry for it (index 0 still points at owner).
	require.NoError(t, ak.Accounts.Indexes.Number.Unreference(ctx, claimAddr, func() (sdk.AccountI, error) {
		return claimAcc, nil
	}))
	require.NoError(t, claimAcc.SetAccountNumber(0))
	bz, err := app.AppCodec().MarshalInterface(claimAcc)
	require.NoError(t, err)
	key := append(append([]byte{}, authtypes.AddressStoreKeyPrefix.Bytes()...), claimAddr...)
	ctx.KVStore(app.GetKey(authtypes.StoreKey)).Set(key, bz)
	require.Equal(t, uint64(0), ak.GetAccount(ctx, claimAddr).GetAccountNumber())

	// Precondition: the state is inconsistent. Re-saving the claim account (any
	// SetAccount on it) silently steals index entry 0 from its owner.
	cacheCtx, _ := ctx.CacheContext()
	ak.SetAccount(cacheCtx, ak.GetAccount(cacheCtx, claimAddr))
	stolen, err := ak.Accounts.Indexes.Number.MatchExact(cacheCtx, 0)
	require.NoError(t, err)
	require.True(t, stolen.Equals(claimAddr), "precondition: re-saving the duplicate steals index entry 0")

	// Run the real v4.1.0 upgrade handler.
	handler := v410.CreateUpgradeHandler(
		app.mm, app.configurator, app.DistrKeeper, app.BankKeeper, app.AccountKeeper, app.ClaimKeeper,
		app.ConsensusParamsKeeper, app.ParamsKeeper, app.StakingKeeper, app.GovKeeper,
	)
	fromVM, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.NoError(t, err)
	toVM, err := handler(ctx, upgradetypes.Plan{Name: v410.Name, Height: ctx.BlockHeight()}, fromVM)
	require.NoError(t, err)
	require.Equal(t, fromVM, toVM, "v4.1.0 must not bump any module consensus version")

	repaired := ak.GetAccount(ctx, claimAddr)
	require.NotEqual(t, uint64(0), repaired.GetAccountNumber())
	idxOwner, err := ak.Accounts.Indexes.Number.MatchExact(ctx, 0)
	require.NoError(t, err)
	require.True(t, idxOwner.Equals(owner), "account number 0 must stay with its original owner")
	idxClaim, err := ak.Accounts.Indexes.Number.MatchExact(ctx, repaired.GetAccountNumber())
	require.NoError(t, err)
	require.True(t, idxClaim.Equals(claimAddr))
	requireUniqueAccountNumbers(t, app, ctx)

	// Idempotent.
	changed, err := v410.FixDuplicateAccountNumbers(ctx, ak)
	require.NoError(t, err)
	require.Empty(t, changed)

	// The repaired state can be exported and imported, keeping the account numbers.
	fixed, err := app.mm.ExportGenesisForModules(ctx, app.AppCodec(), nil)
	require.NoError(t, err)
	importApp := NewTestApp(0)
	importCtx := importApp.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight()})
	require.NotPanics(t, func() {
		_, err = importApp.mm.InitGenesis(importCtx, importApp.AppCodec(), fixed)
	})
	require.NoError(t, err)
	require.Equal(t, repaired.GetAccountNumber(),
		importApp.AccountKeeper.GetAccount(importCtx, claimAddr).GetAccountNumber())
	requireUniqueAccountNumbers(t, importApp, importCtx)
}

// x/claim params live in an x/params subspace; governance can only change them
// through a legacy ParameterChangeProposal, which needs the legacy gov router.
func TestLegacyParamChangeProposalUpdatesClaimParams(t *testing.T) {
	app := Setup(false, t)
	ctx := deliverCtx(app)

	require.NotNil(t, app.GovKeeper.LegacyRouter(), "legacy gov router must be registered")

	content := paramproposal.NewParameterChangeProposal("claim denom", "change claim denom", []paramproposal.ParamChange{
		{Subspace: claimtypes.ModuleName, Key: string(claimtypes.KeyClaimDenom), Value: `"uclaimtest"`},
	})
	contentAny, err := codectypes.NewAnyWithValue(content)
	require.NoError(t, err)
	msg := govv1.NewMsgExecLegacyContent(contentAny, authtypes.NewModuleAddress(govtypes.ModuleName).String())

	handler := app.MsgServiceRouter().Handler(msg)
	require.NotNil(t, handler)
	_, err = handler(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, "uclaimtest", app.ClaimKeeper.GetParams(ctx).ClaimDenom)

	// A text proposal (v1beta1 content) must also be routable.
	require.True(t, app.GovKeeper.LegacyRouter().HasRoute(govtypes.RouterKey))
}

// The module version map stored by InitChain matches every module's consensus version.
func TestUpgradeStateOnGenesis(t *testing.T) {
	app := Setup(false, t)
	ctx := deliverCtx(app)
	vm, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.NoError(t, err)
	for name, mod := range app.mm.Modules {
		if cv, ok := mod.(module.HasConsensusVersion); ok {
			require.Equal(t, cv.ConsensusVersion(), vm[name], name)
		}
	}
}

// The registered v4.1.0 handler runs through the upgrade keeper and keeps module
// versions, wasm params and the supply unchanged.
func TestSecurityUpgrade(t *testing.T) {
	app := Setup(false, t)
	ctx := deliverCtx(app)
	require.True(t, app.UpgradeKeeper.HasHandler(v410.Name))
	before, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.NoError(t, err)
	wasmBefore := app.WasmKeeper.GetParams(ctx)
	supplyBefore := app.BankKeeper.GetSupply(ctx, "upasg")
	plan := upgradetypes.Plan{Name: v410.Name, Height: ctx.BlockHeight() + 1}
	require.NoError(t, app.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	upgradeCtx := ctx.WithBlockHeight(plan.Height).WithHeaderInfo(header.Info{Height: plan.Height})
	require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(upgradeCtx, plan))
	after, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, wasmBefore, app.WasmKeeper.GetParams(ctx))
	require.Equal(t, supplyBefore, app.BankKeeper.GetSupply(ctx, "upasg"))
	height, err := app.UpgradeKeeper.GetDoneHeight(ctx, plan.Name)
	require.NoError(t, err)
	require.Equal(t, plan.Height, height)
}
