package app

import (
	storetypes "cosmossdk.io/store/types"
	circuitante "cosmossdk.io/x/circuit/ante"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	ibcante "github.com/cosmos/ibc-go/v8/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	passageante "github.com/envadiv/Passage3D/app/ante"
)

// HandlerOptions are the options required for constructing a default SDK AnteHandler.
type HandlerOptions struct {
	ante.HandlerOptions
	TxCounterStoreKey storetypes.StoreKey
	WasmConfig        wasmtypes.WasmConfig
	IBCKeeper         *ibckeeper.Keeper
	Codec             codec.Codec
	CircuitKeeper     circuitante.CircuitBreaker
}

// NewAnteHandler returns an AnteHandler that checks and increments sequence
// numbers, checks signatures & account numbers, and deducts fees from the first
// signer.
func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, error) {
	if options.AccountKeeper == nil {
		return nil, sdkerrors.ErrLogic.Wrap("account keeper is required for ante builder")
	}

	if options.BankKeeper == nil {
		return nil, sdkerrors.ErrLogic.Wrap("bank keeper is required for ante builder")
	}

	if options.SignModeHandler == nil {
		return nil, sdkerrors.ErrLogic.Wrap("sign mode handler is required for ante builder")
	}

	sigGasConsumer := options.SigGasConsumer
	if sigGasConsumer == nil {
		sigGasConsumer = ante.DefaultSigVerificationGasConsumer
	}

	anteDecorators := []sdk.AnteDecorator{
		ante.NewSetUpContextDecorator(), // outermost AnteDecorator. SetUpContext must be called first
		circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),
		wasmkeeper.NewLimitSimulationGasDecorator(options.WasmConfig.SimulationGasLimit),
		wasmkeeper.NewCountTXDecorator(runtime.NewKVStoreService(options.TxCounterStoreKey.(*storetypes.KVStoreKey))),
		ante.NewExtensionOptionsDecorator(nil),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		ante.NewDeductFeeDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper, nil),
		passageante.NewBlockAccountDecorator(options.Codec),
		// SetPubKeyDecorator must be called before all signature verification decorators
		ante.NewSetPubKeyDecorator(options.AccountKeeper),
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		ante.NewSigGasConsumeDecorator(options.AccountKeeper, sigGasConsumer),
		ante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler),
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),
		ibcante.NewRedundantRelayDecorator(options.IBCKeeper),
	}

	return sdk.ChainAnteDecorators(anteDecorators...), nil
}
