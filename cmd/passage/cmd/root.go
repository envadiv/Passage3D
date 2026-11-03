package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	confixcmd "cosmossdk.io/tools/confix/cmd"
	tmcfg "github.com/cometbft/cometbft/config"
	tmcli "github.com/cometbft/cometbft/libs/cli"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	"github.com/CosmWasm/wasmd/x/wasm"
	wasmcli "github.com/CosmWasm/wasmd/x/wasm/client/cli"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	"github.com/cosmos/cosmos-sdk/server"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"

	"github.com/envadiv/Passage3D/app"
	passageante "github.com/envadiv/Passage3D/app/ante"
	appparams "github.com/envadiv/Passage3D/app/params"
)

// DefaultMinGasPrices is the minimum-gas-prices written to a new app.toml. Nodes
// refuse to start with a lower value (see newApp), as in Passage v2.x.
var DefaultMinGasPrices = passageante.DefaultMinGasPrice + passageante.BaseDenom

// basicManager is built once from a throwaway app so every AppModuleBasic
// carries its codecs (0.50: a bare AppModuleBasic{} nil-panics in GetTxCmd).
var basicManager module.BasicManager

type rootAppOptions struct{ home string }

func (o rootAppOptions) Get(k string) interface{} {
	if k == flags.FlagHome {
		return o.home
	}
	return nil
}

// NewRootCmd creates a new root command for passage. It is called once in the
// main function.
func NewRootCmd() (*cobra.Command, appparams.EncodingConfig) {
	encodingConfig := app.MakeEncodingConfig()

	// The throwaway app only provides the module basics and autocli options. Its
	// home (wasm cache) lives in a temp dir that is removed right away.
	tmpHome, err := os.MkdirTemp("", "passage-cli")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpHome)
	tempApp := app.NewPassageApp(
		log.NewNopLogger(), dbm.NewMemDB(), nil, false,
		map[int64]bool{}, tmpHome, uint(1), encodingConfig,
		app.GetEnabledProposals(), rootAppOptions{home: tmpHome}, []wasmkeeper.Option(nil),
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
		WithViper("") // no prefix for env variables

	rootCmd := &cobra.Command{
		Use:           "passage",
		Short:         "Passage daemon (server and client)",
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx = initClientCtx.WithCmdContext(cmd.Context())
			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}

			// SIGN_MODE_TEXTUAL (enabled on the node) needs a client-side tx config
			// that can query coin metadata, so it is only available online. This
			// must run after ReadFromClientConfig, which sets the RPC client.
			if !initClientCtx.Offline {
				enabledSignModes := append(slices.Clone(authtx.DefaultSignModes), signing.SignMode_SIGN_MODE_TEXTUAL)
				txConfig, err := authtx.NewTxConfigWithOptions(initClientCtx.Codec, authtx.ConfigOptions{
					EnabledSignModes:           enabledSignModes,
					TextualCoinMetadataQueryFn: txmodule.NewGRPCCoinMetadataQueryFn(initClientCtx),
				})
				if err != nil {
					return err
				}
				initClientCtx = initClientCtx.WithTxConfig(txConfig)
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

// initAppConfig returns the app.toml template and default values: SDK server
// config + the [wasm] section, with Passage's minimum gas price.
func initAppConfig() (string, interface{}) {
	type CustomAppConfig struct {
		serverconfig.Config

		Wasm wasmtypes.NodeConfig `mapstructure:"wasm"`
	}

	srvCfg := serverconfig.DefaultConfig()
	// Nodes refuse to start below this value (see newApp).
	srvCfg.MinGasPrices = DefaultMinGasPrices

	customAppConfig := CustomAppConfig{
		Config: *srvCfg,
		Wasm:   wasmtypes.DefaultNodeConfig(),
	}

	customAppTemplate := serverconfig.DefaultConfigTemplate + wasmtypes.DefaultConfigTemplate()

	return customAppTemplate, customAppConfig
}

func initRootCmd(rootCmd *cobra.Command, encodingConfig appparams.EncodingConfig) {
	cfg := sdk.GetConfig()
	cfg.Seal()

	a := appCreator{encodingConfig}
	valAddrCodec := authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix())

	rootCmd.AddCommand(
		genutilcli.InitCmd(basicManager, app.DefaultNodeHome),
		// Top-level genesis commands kept for backwards compatibility with v2.x
		// scripts and docs; the same commands are also under `passage genesis`.
		genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, genutiltypes.DefaultMessageValidator, valAddrCodec),
		genutilcli.MigrateGenesisCmd(genutilcli.MigrationMap),
		genutilcli.GenTxCmd(basicManager, encodingConfig.TxConfig, banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, valAddrCodec),
		genutilcli.ValidateGenesisCmd(basicManager),
		AddGenesisAccountCmd(app.DefaultNodeHome),
		genutilcli.Commands(encodingConfig.TxConfig, basicManager, app.DefaultNodeHome),
		tmcli.NewCompletionCmd(rootCmd, true),
		testnetCmd(basicManager, banktypes.GenesisBalancesIterator{}),
		debug.Cmd(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(a.newApp, app.DefaultNodeHome),
		snapshot.Cmd(a.newApp),
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome, a.newApp, a.appExport, addModuleInitFlags)
	addInPlaceTestnetCommand(rootCmd, a)
	// `passage comet unsafe-reset-all` also clears the wasm directory.
	wasmcli.ExtendUnsafeResetAllCmd(rootCmd)

	// Expose CometBFT event re-indexing as `passage comet reindex-event`
	// (validator request post-v4.0.2; CLI-only, no consensus impact).
	for _, c := range rootCmd.Commands() {
		if c.Name() == "comet" {
			c.AddCommand(NewReindexEventCmd())
		}
	}

	// add keybase, auxiliary RPC, query, and tx child commands. Module commands
	// are added by autocli (EnhanceRootCommand), including the hand-written ones.
	rootCmd.AddCommand(
		server.StatusCommand(),
		queryCommand(),
		txCommand(),
		keys.Commands(),
	)
}

func addModuleInitFlags(startCmd *cobra.Command) {
	crisis.AddModuleInitFlags(startCmd)
	// --wasm.* flags; also verifies at start that the linked libwasmvm matches
	// the wasmvm Go module version.
	wasm.AddModuleInitFlags(startCmd)
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.ValidatorCommand(),
		rpc.QueryEventForTxCmd(),
		server.QueryBlockCmd(),
		server.QueryBlocksCmd(),
		server.QueryBlockResultsCmd(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
	)

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
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
		authcmd.GetSimulateCmd(),
	)

	return cmd
}

type appCreator struct {
	encCfg appparams.EncodingConfig
}

// newApp is an appCreator
func (a appCreator) newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	baseappOptions := server.DefaultBaseappOptions(appOpts)

	// Passage minimum gas price floor (12.5upasg). v2.x refused to start below it;
	// v4.0.0-v4.0.3 had no floor and wrote "0upasg" into new app.toml files, so a
	// hard failure would stop such validators at the v4.0.4 upgrade height. Instead
	// the floor is applied (with a warning) on top of the configured prices.
	minGasPrices, err := sdk.ParseDecCoins(cast.ToString(appOpts.Get(server.FlagMinGasPrices)))
	if err != nil {
		panic(fmt.Errorf("invalid minimum-gas-prices: %w", err))
	}
	if err := passageante.ValidateMinGasPrices(minGasPrices); err != nil {
		floor := sdk.NewDecCoinFromDec(passageante.BaseDenom, sdkmath.LegacyMustNewDecFromStr(passageante.DefaultMinGasPrice))
		enforced := minGasPrices.Sub(sdk.NewDecCoins(sdk.NewDecCoinFromDec(passageante.BaseDenom, minGasPrices.AmountOf(passageante.BaseDenom)))).Add(floor)
		logger.Warn("minimum-gas-prices below the Passage floor; using the floor instead",
			"configured", minGasPrices.String(), "enforced", enforced.String())
		baseappOptions = append(baseappOptions, baseapp.SetMinGasPrices(enforced.String()))
	}

	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}

	var wasmOpts []wasmkeeper.Option
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
		baseappOptions...,
	)
}

// appExport creates a new app (optionally at a given height) and exports state.
func (a appCreator) appExport(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailAllowedAddrs []string,
	appOpts servertypes.AppOptions, modulesToExport []string,
) (servertypes.ExportedApp, error) {
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home not set")
	}

	var passageApp *app.PassageApp
	if height != -1 {
		passageApp = app.NewPassageApp(logger, db, traceStore, false, map[int64]bool{}, homePath, uint(1), a.encCfg, app.GetEnabledProposals(), appOpts, nil)
		if err := passageApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		passageApp = app.NewPassageApp(logger, db, traceStore, true, map[int64]bool{}, homePath, uint(1), a.encCfg, app.GetEnabledProposals(), appOpts, nil)
	}

	return passageApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}
