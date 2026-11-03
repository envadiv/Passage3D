// Package network runs an in-process Passage test network. It is a thin
// wrapper around the Cosmos SDK testutil/network that builds every validator
// with a real PassageApp.
package network

import (
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	pruningtypes "cosmossdk.io/store/pruning/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdknetwork "github.com/cosmos/cosmos-sdk/testutil/network"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"

	"github.com/envadiv/Passage3D/app"
	passageante "github.com/envadiv/Passage3D/app/ante"
	"github.com/envadiv/Passage3D/app/params"
)

type (
	// Network is an in-process test network.
	Network = sdknetwork.Network
	// Config configures a Network.
	Config = sdknetwork.Config
	// Validator is a validator node of a Network.
	Validator = sdknetwork.Validator
)

// MinGasPrices is the lowest minimum-gas-prices a Passage node accepts.
var MinGasPrices = passageante.DefaultMinGasPrice + passageante.BaseDenom

// NewAppConstructor returns an AppConstructor that builds a PassageApp on an
// in-memory database, using the validator's node directory as app home (the
// wasm cache lives there). Like `passage start`, it refuses minimum gas prices
// below the Passage floor.
func NewAppConstructor(encCfg params.EncodingConfig) sdknetwork.AppConstructor {
	return func(val sdknetwork.ValidatorI) servertypes.Application {
		appCfg := val.GetAppConfig()
		minGasPrices, err := sdk.ParseDecCoins(appCfg.MinGasPrices)
		if err != nil {
			panic(err)
		}
		if err := passageante.ValidateMinGasPrices(minGasPrices); err != nil {
			panic(err)
		}

		srvCtx := val.GetCtx()
		return app.NewPassageApp(
			srvCtx.Logger, dbm.NewMemDB(), nil, true, map[int64]bool{},
			srvCtx.Config.RootDir, 0, encCfg, app.GetEnabledProposals(), srvCtx.Viper, nil,
			baseapp.SetPruning(pruningtypes.NewPruningOptionsFromString(appCfg.Pruning)),
			baseapp.SetMinGasPrices(appCfg.MinGasPrices),
			baseapp.SetChainID(srvCtx.Viper.GetString(flags.FlagChainID)),
		)
	}
}

// DefaultConfig returns a network configuration for PassageApp with the
// default genesis state, upasg as bond denom and the Passage minimum gas price.
func DefaultConfig() Config {
	encCfg := app.MakeEncodingConfig()
	cfg := sdknetwork.DefaultConfig(func() sdknetwork.TestFixture {
		return sdknetwork.TestFixture{
			AppConstructor: NewAppConstructor(encCfg),
			GenesisState:   app.NewDefaultGenesisState(encCfg.Marshaler),
			EncodingConfig: moduletestutil.TestEncodingConfig{
				InterfaceRegistry: encCfg.InterfaceRegistry,
				Codec:             encCfg.Marshaler,
				TxConfig:          encCfg.TxConfig,
				Amino:             encCfg.Amino,
			},
		}
	})
	cfg.BondDenom = app.BondDenom
	cfg.MinGasPrices = MinGasPrices
	return cfg
}

// New starts a network in a temporary directory and waits for its first
// block. Only one network can run at a time: callers must call Cleanup once
// they are done with it.
func New(t *testing.T, cfg Config) *Network {
	t.Helper()
	net, err := sdknetwork.New(t, t.TempDir(), cfg)
	require.NoError(t, err)
	_, err = net.WaitForHeight(1)
	require.NoError(t, err)
	return net
}
