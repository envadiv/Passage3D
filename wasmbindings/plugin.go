package wasmbindings

import (
	wasmKeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"

	"github.com/envadiv/Passage3D/wasmbindings/claim"
)

// ClaimKeeperExpected is the expected x/claim keeper.
type ClaimKeeperExpected interface {
	claim.KeeperWriterExpected
	// claim.KeeperReaderExpected
}

// BuildWasmOptions returns x/wasmd module options to support WASM bindings functionality.
func BuildWasmOptions(rKeeper ClaimKeeperExpected) []wasmKeeper.Option {
	return []wasmKeeper.Option{
		wasmKeeper.WithMessageHandlerDecorator(BuildWasmMsgDecorator(rKeeper)),
		wasmKeeper.WithQueryPlugins(BuildWasmQueryPlugin(rKeeper)),
	}
}

// BuildWasmMsgDecorator returns the Wasm custom message handler decorator.
func BuildWasmMsgDecorator(rKeeper ClaimKeeperExpected) func(old wasmKeeper.Messenger) wasmKeeper.Messenger {
	return func(old wasmKeeper.Messenger) wasmKeeper.Messenger {
		return NewMsgDispatcher(
			old,
			claim.NewClaimMsgHandler(rKeeper),
		)
	}
}

// BuildWasmQueryPlugin returns the Wasm custom querier plugin.
// func BuildWasmQueryPlugin(rKeeper ClaimKeeperExpected) *wasmKeeper.QueryPlugins {
// 	return &wasmKeeper.QueryPlugins{
// 		Custom: NewQueryDispatcher(
// 			claim.NewQueryHandler(rKeeper),
// 		).DispatchQuery,
// 	}
// }