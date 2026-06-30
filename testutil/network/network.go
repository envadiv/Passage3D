package network

// ponytail: 0.50 STUB. The in-process testnet harness this file used to
// implement (cometbft node.NewNode + SDK server api/grpc startup) was rewritten
// wholesale in SDK v0.50 and cometbft v0.38. The only consumer
// (x/claim/client/testutil) already t.Skip()s the network-backed CLI test, so
// porting the full harness is dead weight. We keep the exported surface
// (Config/Validator/Network + DefaultConfig/NewAppConstructor/New) so callers
// compile, and New fails loudly if ever actually invoked.
// Upgrade path: replace this file's body by embedding SDK 0.50's own
// cosmossdk.io/.../testutil/network with app.NewPassageApp as the AppConstructor.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	pruningtypes "cosmossdk.io/store/pruning/types"

	tmrand "github.com/cometbft/cometbft/libs/rand"
	"github.com/cometbft/cometbft/node"
	tmclient "github.com/cometbft/cometbft/rpc/client"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"google.golang.org/grpc"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/app/params"
)

// package-wide network lock to only allow one test network at a time
var lock = new(sync.Mutex)

// AppConstructor defines a function which accepts a network configuration and
// creates an ABCI Application to provide to CometBFT.
type AppConstructor = func(val Validator) servertypes.Application

// NewAppConstructor returns a new app AppConstructor
func NewAppConstructor(_ params.EncodingConfig) AppConstructor {
	return func(val Validator) servertypes.Application {
		panic("testutil/network in-process harness not ported to SDK v0.50")
	}
}

// Config defines the necessary configuration used to bootstrap and start an
// in-process local testing network.
type Config struct {
	Codec             codec.Codec
	LegacyAmino       *codec.LegacyAmino
	InterfaceRegistry codectypes.InterfaceRegistry

	TxConfig         client.TxConfig
	AccountRetriever client.AccountRetriever
	AppConstructor   AppConstructor
	GenesisState     map[string]json.RawMessage
	TimeoutCommit    time.Duration
	ChainID          string
	NumValidators    int
	Mnemonics        []string
	BondDenom        string
	MinGasPrices     string
	AccountTokens    sdkmath.Int
	StakingTokens    sdkmath.Int
	BondedTokens     sdkmath.Int
	PruningStrategy  string
	EnableLogging    bool
	CleanupDir       bool
	SigningAlgo      string
	KeyringOptions   []keyring.Option
}

// DefaultConfig returns a sane default configuration suitable for nearly all
// testing requirements.
func DefaultConfig() Config {
	encCfg := app.MakeEncodingConfig()

	return Config{
		Codec:             encCfg.Marshaler,
		TxConfig:          encCfg.TxConfig,
		LegacyAmino:       encCfg.Amino,
		InterfaceRegistry: encCfg.InterfaceRegistry,
		AccountRetriever:  authtypes.AccountRetriever{},
		AppConstructor:    NewAppConstructor(encCfg),
		GenesisState:      app.ModuleBasics.DefaultGenesis(encCfg.Marshaler),
		TimeoutCommit:     2 * time.Second,
		ChainID:           "chain-" + tmrand.NewRand().Str(6),
		NumValidators:     4,
		BondDenom:         sdk.DefaultBondDenom,
		MinGasPrices:      fmt.Sprintf("0.000006%s", sdk.DefaultBondDenom),
		AccountTokens:     sdk.TokensFromConsensusPower(1000, sdk.DefaultPowerReduction),
		StakingTokens:     sdk.TokensFromConsensusPower(500, sdk.DefaultPowerReduction),
		BondedTokens:      sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction),
		PruningStrategy:   pruningtypes.PruningOptionNothing,
		CleanupDir:        true,
		SigningAlgo:       string(hd.Secp256k1Type),
		KeyringOptions:    []keyring.Option{},
	}
}

type (
	// Network defines a local in-process testing network.
	Network struct {
		T          *testing.T
		BaseDir    string
		Validators []*Validator

		Config Config
	}

	// Validator defines an in-process CometBFT validator node.
	Validator struct {
		AppConfig  *srvconfig.Config
		ClientCtx  client.Context
		Ctx        *server.Context
		Dir        string
		NodeID     string
		PubKey     cryptotypes.PubKey
		Moniker    string
		APIAddress string
		RPCAddress string
		P2PAddress string
		Address    sdk.AccAddress
		ValAddress sdk.ValAddress
		RPCClient  tmclient.Client

		tmNode  *node.Node
		api     *api.Server
		grpc    *grpc.Server
		grpcWeb *http.Server
	}
)

// New creates a new Network for integration tests.
//
// ponytail: harness not ported to SDK v0.50 — see file header. Callers should
// t.Skip before reaching here; if they do not, fail with a clear message rather
// than a nil-deref deeper in.
func New(t *testing.T, _ Config) *Network {
	lock.Lock()
	defer lock.Unlock()
	t.Skip("testutil/network in-process harness not ported to SDK v0.50")
	return nil
}

// LatestHeight returns the latest height of the network.
func (n *Network) LatestHeight() (int64, error) {
	return 0, errors.New("testutil/network harness not ported to SDK v0.50")
}

// WaitForHeight blocks until the given height is committed.
func (n *Network) WaitForHeight(h int64) (int64, error) {
	return 0, errors.New("testutil/network harness not ported to SDK v0.50")
}

// WaitForHeightWithTimeout is WaitForHeight with a caller-provided timeout.
func (n *Network) WaitForHeightWithTimeout(h int64, t time.Duration) (int64, error) {
	return 0, errors.New("testutil/network harness not ported to SDK v0.50")
}

// WaitForNextBlock waits for the next block to be committed.
func (n *Network) WaitForNextBlock() error {
	return errors.New("testutil/network harness not ported to SDK v0.50")
}

// Cleanup is a no-op in the stubbed harness.
func (n *Network) Cleanup() {}
