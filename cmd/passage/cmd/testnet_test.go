package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltest "github.com/cosmos/cosmos-sdk/x/genutil/client/testutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/envadiv/Passage3D/app"
)

func Test_TestnetCmd(t *testing.T) {
	home := t.TempDir()
	encodingConfig := app.MakeEncodingConfig()
	cfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)

	err = genutiltest.ExecInitCmd(app.ModuleBasics, home, encodingConfig.Marshaler)
	require.NoError(t, err)

	serverCtx := server.NewContext(viper.New(), cfg, log.NewNopLogger())
	clientCtx := client.Context{}.
		WithCodec(encodingConfig.Marshaler).
		WithHomeDir(home).
		WithTxConfig(encodingConfig.TxConfig)

	ctx := context.Background()
	ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	cmd := testnetCmd(app.ModuleBasics, banktypes.GenesisBalancesIterator{})
	cmd.SetArgs([]string{fmt.Sprintf("--%s=test", flags.FlagKeyringBackend), fmt.Sprintf("--output-dir=%s", home)})
	err = cmd.ExecuteContext(ctx)
	require.NoError(t, err)

	genFile := cfg.GenesisFile()
	appState, _, err := genutiltypes.GenesisStateFromGenFile(genFile)
	require.NoError(t, err)

	bankGenState := banktypes.GetGenesisStateFromAppState(encodingConfig.Marshaler, appState)
	require.NotEmpty(t, bankGenState.Supply.String())
}

// TestTestnetCmdGenesisIsValid runs `passage testnet --v 2` through the root
// command and checks that the generated genesis passes the same validation as
// `passage genesis validate` and uses Passage's denom and minimum commission.
func TestTestnetCmdGenesisIsValid(t *testing.T) {
	home, outputDir := t.TempDir(), t.TempDir()
	rootCmd, encCfg := NewRootCmd()
	rootCmd.SetArgs([]string{
		"testnet",
		fmt.Sprintf("--%s=2", flagNumValidators),
		fmt.Sprintf("--%s=%s", flagOutputDir, outputDir),
		fmt.Sprintf("--%s=%s", flags.FlagKeyringBackend, "test"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "passage-testnet-1"),
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
	})
	require.NoError(t, svrcmd.Execute(rootCmd, "", home))

	var genesisFiles [][]byte
	for i := 0; i < 2; i++ {
		genFile := filepath.Join(outputDir, fmt.Sprintf("node%d", i), "passage", "config", "genesis.json")
		bz, err := os.ReadFile(genFile)
		require.NoError(t, err)
		genesisFiles = append(genesisFiles, bz)

		appGenesis, err := genutiltypes.AppGenesisFromFile(genFile)
		require.NoError(t, err)
		require.NoError(t, appGenesis.ValidateAndComplete())
		require.Equal(t, "passage-testnet-1", appGenesis.ChainID)

		var appState map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(appGenesis.AppState, &appState))
		require.NoError(t, basicManager.ValidateGenesis(encCfg.Marshaler, encCfg.TxConfig, appState))

		stakingGenState := stakingtypes.GetGenesisStateFromAppState(encCfg.Marshaler, appState)
		require.Equal(t, app.BondDenom, stakingGenState.Params.BondDenom)
		require.Equal(t, sdkmath.LegacyNewDecWithPrec(5, 2), stakingGenState.Params.MinCommissionRate)

		genTxs := genutiltypes.GetGenesisStateFromAppState(encCfg.Marshaler, appState).GenTxs
		require.Len(t, genTxs, 2)

		// Every module default (mint, gov deposits, crisis fee, ...) must use
		// upasg rather than the SDK's "stake".
		require.NotContains(t, string(appGenesis.AppState), `"stake"`)
	}
	require.JSONEq(t, string(genesisFiles[0]), string(genesisFiles[1]), "validators must share the same genesis")

	// The chain boots from it: InitChain delivers both gentxs.
	appGenesis, err := genutiltypes.AppGenesisFromFile(filepath.Join(outputDir, "node0", "passage", "config", "genesis.json"))
	require.NoError(t, err)
	require.NoError(t, appGenesis.ValidateAndComplete())
	consensusParams := appGenesis.Consensus.Params.ToProto()
	passageApp := app.NewPassageApp(
		log.NewNopLogger(), dbm.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 0, encCfg, nil,
		app.EmptyAppOptions{}, nil, baseapp.SetChainID(appGenesis.ChainID),
	)
	res, err := passageApp.InitChain(&abci.RequestInitChain{
		Time:            appGenesis.GenesisTime,
		ChainId:         appGenesis.ChainID,
		InitialHeight:   appGenesis.InitialHeight,
		ConsensusParams: &consensusParams,
		AppStateBytes:   appGenesis.AppState,
	})
	require.NoError(t, err)
	require.Len(t, res.Validators, 2)
}
