package app

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/feegrant"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	simcli "github.com/cosmos/cosmos-sdk/x/simulation/client/cli"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Simulation tests (ported from the Cosmos SDK / wasmd v0.50 simapp). They are
// skipped unless -Enabled=true is passed, e.g.:
//
//	go test ./app -run TestFullAppSimulation -Enabled=true -NumBlocks=100 -BlockSize=50 -Commit=true -Seed=42 -v -timeout 1h
//
// See the test-sim-* targets in the Makefile.

// SimAppChainID hardcoded chainID for simulation
const SimAppChainID = "simulation-app"

func init() {
	simcli.GetSimulatorFlags()
}

// NOTE: the SDK simapp runs these simulations in "faux merkle" mode (raw DB
// stores instead of IAVL). It is not used here: with goleveldb a zero-length
// value can be read back as nil, so an empty x/distribution FeePool written at
// genesis is reported as "collections: not found" in faux merkle mode. Passage
// nodes always run IAVL stores, so the simulations use them too.

// interBlockCacheOpt returns a BaseApp option function that sets the persistent
// inter-block write-through cache.
func interBlockCacheOpt() func(*baseapp.BaseApp) {
	return baseapp.SetInterBlockCache(store.NewCommitKVStoreCacheManager())
}

func newSimApp(logger log.Logger, db dbm.DB, home string, appOpts simtestutil.AppOptionsMap, opts ...func(*baseapp.BaseApp)) *PassageApp {
	return NewPassageApp(
		logger, db, nil, true, map[int64]bool{}, home, simcli.FlagPeriodValue,
		MakeEncodingConfig(), nil, appOpts, emptyWasmOptions, opts...,
	)
}

func setupSimulationApp(t *testing.T, msg string) (simtypes.Config, dbm.DB, simtestutil.AppOptionsMap, *PassageApp) {
	t.Helper()
	config := simcli.NewConfigFromFlags()
	config.ChainID = SimAppChainID

	db, dir, logger, skip, err := simtestutil.SetupSimulation(config, "leveldb-app-sim", "Simulation", simcli.FlagVerboseValue, simcli.FlagEnabledValue)
	if skip {
		t.Skip(msg)
	}
	require.NoError(t, err, "simulation setup failed")
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dir))
	})

	appOptions := make(simtestutil.AppOptionsMap, 0)
	appOptions[flags.FlagHome] = dir // ensure a unique folder
	appOptions[server.FlagInvCheckPeriod] = simcli.FlagPeriodValue

	app := newSimApp(logger, db, dir, appOptions, baseapp.SetChainID(SimAppChainID))
	require.Equal(t, appName, app.Name())
	return config, db, appOptions, app
}

func runSimulation(t *testing.T, app *PassageApp, config simtypes.Config) (bool, simulation.Params) {
	t.Helper()
	stopEarly, simParams, simErr := simulation.SimulateFromSeed(
		t,
		os.Stdout,
		app.BaseApp,
		simtestutil.AppStateFn(app.AppCodec(), app.SimulationManager(), app.DefaultGenesis()),
		simtypes.RandomAccounts,
		simtestutil.SimulationOperations(app, app.AppCodec(), config),
		BlockedAddresses(),
		config,
		app.AppCodec(),
	)
	require.NoError(t, simErr)
	return stopEarly, simParams
}

func TestFullAppSimulation(t *testing.T) {
	config, db, _, app := setupSimulationApp(t, "skipping application simulation")

	_, simParams := runSimulation(t, app, config)
	require.NoError(t, simtestutil.CheckExportSimulation(app, config, simParams))
	if config.Commit {
		simtestutil.PrintStats(db)
	}
}

func TestAppImportExport(t *testing.T) {
	config, db, appOptions, app := setupSimulationApp(t, "skipping application import/export simulation")

	_, simParams := runSimulation(t, app, config)
	require.NoError(t, simtestutil.CheckExportSimulation(app, config, simParams))
	if config.Commit {
		simtestutil.PrintStats(db)
	}

	t.Log("exporting genesis...")
	exported, err := app.ExportAppStateAndValidators(false, []string{}, []string{})
	require.NoError(t, err)

	t.Log("importing genesis...")
	newDB, newDir, _, _, err := simtestutil.SetupSimulation(config, "leveldb-app-sim-2", "Simulation-2", simcli.FlagVerboseValue, simcli.FlagEnabledValue)
	require.NoError(t, err, "simulation setup failed")
	defer func() {
		require.NoError(t, newDB.Close())
		require.NoError(t, os.RemoveAll(newDir))
	}()
	newApp := newSimApp(log.NewNopLogger(), newDB, newDir, appOptions, baseapp.SetChainID(SimAppChainID))

	initReq := &abci.RequestInitChain{AppStateBytes: exported.AppState}
	ctxA := app.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight()})
	ctxB := newApp.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight()})
	_, err = newApp.InitChainer(ctxB, initReq)
	if err != nil && strings.Contains(err.Error(), "validator set is empty after InitGenesis") {
		t.Logf("Skipping simulation as all validators have been unbonded: %s\n%s", err, debug.Stack())
		return
	}
	require.NoError(t, err)
	require.NoError(t, newApp.StoreConsensusParams(ctxB, exported.ConsensusParams))

	t.Log("comparing stores...")
	// skip prefixes holding data that is legitimately rebuilt or dropped on import
	skipPrefixes := map[string][][]byte{
		stakingtypes.StoreKey: {
			stakingtypes.UnbondingQueueKey, stakingtypes.RedelegationQueueKey, stakingtypes.ValidatorQueueKey,
			stakingtypes.HistoricalInfoKey, stakingtypes.UnbondingIDKey, stakingtypes.UnbondingIndexKey,
			stakingtypes.UnbondingTypeKey, stakingtypes.ValidatorUpdatesKey,
		},
		authzkeeper.StoreKey:   {authzkeeper.GrantQueuePrefix},
		feegrant.StoreKey:      {feegrant.FeeAllowanceQueueKeyPrefix},
		slashingtypes.StoreKey: {slashingtypes.ValidatorMissedBlockBitmapKeyPrefix},
		wasmtypes.StoreKey:     {wasmtypes.TXCounterPrefix},
	}

	storeKeys := app.GetStoreKeys()
	sort.Slice(storeKeys, func(i, j int) bool { return storeKeys[i].Name() < storeKeys[j].Name() })
	require.NotEmpty(t, storeKeys)
	for _, appKeyA := range storeKeys {
		if _, ok := appKeyA.(*storetypes.KVStoreKey); !ok {
			continue
		}
		keyName := appKeyA.Name()
		storeA := ctxA.KVStore(appKeyA)
		storeB := ctxB.KVStore(newApp.GetKey(keyName))

		failedKVAs, failedKVBs := simtestutil.DiffKVStores(storeA, storeB, skipPrefixes[keyName])
		if !assert.Equal(t, len(failedKVAs), len(failedKVBs), "unequal sets of key-values to compare in %q", keyName) {
			for _, v := range failedKVBs {
				t.Logf("store mismatch: %q\n", v)
			}
			t.FailNow()
		}
		t.Logf("compared %d different key/value pairs between %s and %s\n", len(failedKVAs), keyName, keyName)
		if !assert.Equal(t, 0, len(failedKVAs), simtestutil.GetSimulationLog(keyName, app.SimulationManager().StoreDecoders, failedKVAs, failedKVBs)) {
			for _, v := range failedKVAs {
				t.Logf("store mismatch: %q\n", v)
			}
			t.FailNow()
		}
	}
}

func TestAppSimulationAfterImport(t *testing.T) {
	config, db, appOptions, app := setupSimulationApp(t, "skipping application simulation after import")

	stopEarly, simParams := runSimulation(t, app, config)
	require.NoError(t, simtestutil.CheckExportSimulation(app, config, simParams))
	if config.Commit {
		simtestutil.PrintStats(db)
	}
	if stopEarly {
		t.Log("can't export or import a zero-validator genesis, exiting test...")
		return
	}

	t.Log("exporting genesis (zero height)...")
	exported, err := app.ExportAppStateAndValidators(true, []string{}, []string{})
	require.NoError(t, err)

	t.Log("importing genesis...")
	newDB, newDir, _, _, err := simtestutil.SetupSimulation(config, "leveldb-app-sim-2", "Simulation-2", simcli.FlagVerboseValue, simcli.FlagEnabledValue)
	require.NoError(t, err, "simulation setup failed")
	defer func() {
		require.NoError(t, newDB.Close())
		require.NoError(t, os.RemoveAll(newDir))
	}()
	newApp := newSimApp(log.NewNopLogger(), newDB, newDir, appOptions, baseapp.SetChainID(SimAppChainID))

	_, err = newApp.InitChain(&abci.RequestInitChain{
		ChainId:       SimAppChainID,
		AppStateBytes: exported.AppState,
	})
	require.NoError(t, err)

	runSimulation(t, newApp, config)
}

// TestAppStateDeterminism runs the same simulation several times per seed and
// requires identical app hashes.
func TestAppStateDeterminism(t *testing.T) {
	if !simcli.FlagEnabledValue {
		t.Skip("skipping application simulation")
	}

	config := simcli.NewConfigFromFlags()
	config.InitialBlockHeight = 1
	config.ExportParamsPath = ""
	config.OnOperation = false
	config.AllInvariants = false
	config.ChainID = SimAppChainID

	numSeeds := 3
	numTimesToRunPerSeed := 3
	if config.Seed != simcli.DefaultSeedValue {
		numSeeds = 1
	}
	appHashList := make([]json.RawMessage, numTimesToRunPerSeed)

	for i := 0; i < numSeeds; i++ {
		config.Seed += int64(i)
		for j := 0; j < numTimesToRunPerSeed; j++ {
			logger := log.NewNopLogger()
			if simcli.FlagVerboseValue {
				logger = log.NewTestLogger(t)
			}
			home := t.TempDir()
			appOptions := simtestutil.AppOptionsMap{
				flags.FlagHome:            home,
				server.FlagInvCheckPeriod: simcli.FlagPeriodValue,
			}
			db := dbm.NewMemDB()
			app := newSimApp(logger, db, home, appOptions, interBlockCacheOpt(), baseapp.SetChainID(SimAppChainID))

			fmt.Printf("running non-determinism simulation; seed %d: %d/%d, attempt: %d/%d\n",
				config.Seed, i+1, numSeeds, j+1, numTimesToRunPerSeed)

			runSimulation(t, app, config)
			if config.Commit {
				simtestutil.PrintStats(db)
			}

			appHashList[j] = app.LastCommitID().Hash
			if j != 0 {
				require.Equal(t, string(appHashList[0]), string(appHashList[j]),
					"non-determinism in seed %d: %d/%d, attempt: %d/%d\n", config.Seed, i+1, numSeeds, j+1, numTimesToRunPerSeed)
			}
		}
	}
}
