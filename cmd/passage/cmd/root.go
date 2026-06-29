package cmd

import (
	"errors"
	"io"
	"os"

	appparams "github.com/envadiv/Passage3D/app/params"

	"cosmossdk.io/log"
	tmcfg "github.com/cometbft/cometbft/config"
	tmcli "github.com/cometbft/cometbft/libs/cli"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"

	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/envadiv/Passage3D/app"

	"github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/prometheus/client_golang/prometheus"
)

// basicManager is built once from a throwaway app so every AppModuleBasic
// carries its codecs (0.50 â a bare AppModuleBasic{} nil-panics in GetTxCmd).
var basicManager module.BasicManager

type rootAppOptions struct{ home string }

func (o rootAppOptions) Get(k string) interface{} {
	if k == flags.FlagHome {
		return o.home
	}
	return nil
}

func rootTempDir() string {
	dir, err := os.MkdirTemp("", "passage-cli")
	if err != nil {
		return app.DefaultNodeHome
	}
	return dir
}

// NewRootCmd creates a new root command for simd. It is called once in the
// main function.
func NewRootCmd() (*cobra.Command, appparams.EncodingConfig) {
	encodingConfig := app.MakeEncodingConfig()

	tmpHome := rootTempDir()
	tempApp := app.NewPassageApp(
		log.NewNopLogger(), dbm.NewMemDB(), nil, false,
		map[int64]bool{}, tmpHome, uint(1), encodingConfig,
		app.GetEnabledProposals(), rootAppOptions{home: tmpHome}, []wasm.Option(nil),
	)
	basicManager = tempApp.BasicModuleManager

	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Marshaler).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithViper("") // In simapp, we don't use any prefix for env variables.

	rootCmd := &cobra.Command{
		Use:   "passage",
		Short: "passage app",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			customAppTemplate, customAppConfig := initAppConfig()

			return server.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, tmcfg.DefaultConfig())
		},
	}

	initRootCmd(rootCmd, encodingConfig)

	autoCliOpts := tempApp.AutoCliOpts()
	autoCliOpts.ClientCtx = initClientCtx
	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	return rootCmd, encodingConfig
}

// initAppConfig helps to override default appConfig template and configs.
// return "", nil if no custom configuration is required for the application.
func initAppConfig() (string, interface{}) {
	// The following code snippet is just for reference.

	// WASMConfig defines configuration for the wasm module.
	type WASMConfig struct {
		// This is the maximum sdk gas (wasm and storage) that we allow for any x/wasm "smart" queries
		QueryGasLimit uint64 `mapstructure:"query_gas_limit"`

		// Address defines the gRPC-web server to listen on
		LruSize uint64 `mapstructure:"lru_size"`
	}

	type CustomAppConfig struct {
		serverconfig.Config

		WASM WASMConfig `mapstructure:"wasm"`
	}

	// Optionally allow the chain developer to overwrite the SDK's default
	// server config.
	srvCfg := serverconfig.DefaultConfig()
	// The SDK's default minimum gas price is set to "" (empty value) inside
	// app.toml. If left empty by validators, the node will halt on startup.
	// However, the chain developer can set a default app.toml value for their
	// validators here.
	//
	// In summary:
	// - if you leave srvCfg.MinGasPrices = "", all validators MUST tweak their
	//   own app.toml config,
	// - if you set srvCfg.MinGasPrices non-empty, validators CAN tweak their
	//   own app.toml to override, or use this default value.
	//
	// In simapp, we set the min gas prices to 0.
	srvCfg.MinGasPrices = "0upasg"

	customAppConfig := CustomAppConfig{
		Config: *srvCfg,
		WASM: WASMConfig{
			LruSize:       1,
			QueryGasLimit: 300000,
		},
	}

	customAppTemplate := serverconfig.DefaultConfigTemplate + `
[wasm]
# This is the maximum sdk gas (wasm and storage) that we allow for any x/wasm "smart" queries
query_gas_limit = 300000
# This is the number of wasm vm instances we keep cached in memory for speed-up
# Warning: this is currently unstable and may lead to crashes, best to keep for 0 unless testing locally
lru_size = 0`

	return customAppTemplate, customAppConfig
}

func initRootCmd(rootCmd *cobra.Command, encodingConfig appparams.EncodingConfig) {
	cfg := sdk.GetConfig()
	cfg.Seal()

	rootCmd.AddCommand(
		genutilcli.InitCmd(basicManager, app.DefaultNodeHome),
		genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, genutiltypes.DefaultMessageValidator, authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix())),
		genutilcli.MigrateGenesisCmd(genutilcli.MigrationMap),
		genutilcli.GenTxCmd(basicManager, encodingConfig.TxConfig, banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix())),
		genutilcli.ValidateGenesisCmd(basicManager),
		AddGenesisAccountCmd(app.DefaultNodeHome),
		tmcli.NewCompletionCmd(rootCmd, true),
		testnetCmd(basicManager, banktypes.GenesisBalancesIterator{}),
		debug.Cmd(),
	)

	a := appCreator{encodingConfig}
	server.AddCommands(rootCmd, app.DefaultNodeHome, a.newApp, a.appExport, addModuleInitFlags)

	// add keybase, auxiliary RPC, query, and tx child commands
	rootCmd.AddCommand(
		server.StatusCommand(),
		queryCommand(),
		txCommand(),
		keys.Commands(),
	)

}

func addModuleInitFlags(startCmd *cobra.Command) {
	crisis.AddModuleInitFlags(startCmd)
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.ValidatorCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
	)

	basicManager.AddQueryCommands(cmd)
	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
	)

	basicManager.AddTxCommands(cmd)
	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

type appCreator struct {
	encCfg appparams.EncodingConfig
}

// newApp is an appCreator
func (a appCreator) newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}

	var wasmOpts []wasm.Option
	if cast.ToBool(appOpts.Get("telemetry.enabled")) {
		wasmOpts = append(wasmOpts, wasmkeeper.WithVMCacheMetrics(prometheus.DefaultRegisterer))
	}

	return app.NewPassageApp(
		logger, db, traceStore, true, skipUpgradeHeights,
		cast.ToString(appOpts.Get(flags.FlagHome)),
		cast.ToUint(appOpts.Get(server.FlagInvCheckPeriod)),
		a.encCfg,
		app.GetEnabledProposals(),
		appOpts,
		wasmOpts,
		server.DefaultBaseappOptions(appOpts)...,
	)
}

// appExport creates a new simapp (optionally at a given height)
// and exports state.
func (a appCreator) appExport(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailAllowedAddrs []string,
	appOpts servertypes.AppOptions, modulesToExport []string) (servertypes.ExportedApp, error) {

	var simApp *app.PassageApp
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home not set")
	}
	var emptyWasmOpts []wasm.Option = nil
	if height != -1 {
		simApp = app.NewPassageApp(logger, db, traceStore, false, map[int64]bool{}, homePath, uint(1), a.encCfg, app.GetEnabledProposals(), appOpts, emptyWasmOpts)

		if err := simApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		simApp = app.NewPassageApp(logger, db, traceStore, true, map[int64]bool{}, homePath, uint(1), a.encCfg, app.GetEnabledProposals(), appOpts, emptyWasmOpts)
	}

	return simApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs)
}
