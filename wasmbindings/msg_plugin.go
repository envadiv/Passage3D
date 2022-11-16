package wasmbindings

import (
	"encoding/json"
	"fmt"

	wasmKeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmdTypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmVmTypes "github.com/CosmWasm/wasmvm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/envadiv/Passage3D/wasmbindings/claim"
	"github.com/envadiv/Passage3D/wasmbindings/types"
)

var _ wasmKeeper.Messenger = MsgDispatcher{}

// MsgDispatcher dispatches custom WASM queries.
type MsgDispatcher struct {
	claimHandler   claim.MsgHandler
	wrappedMessenger wasmKeeper.Messenger
}

// NewMsgDispatcher creates a new MsgDispatcher instance.
func NewMsgDispatcher(wrappedMessenger wasmKeeper.Messenger, ch claim.MsgHandler) MsgDispatcher {
	return MsgDispatcher{
		wrappedMessenger: wrappedMessenger,
		claimHandler:   ch,
	}
}

// DispatchMsg validates and executes a custom WASM msg.
func (e MsgDispatcher) DispatchMsg(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg wasmVmTypes.CosmosMsg) ([]sdk.Event, [][]byte, error) {
	// Skip non-custom message
	if msg.Custom == nil {
		return e.wrappedMessenger.DispatchMsg(ctx, contractAddr, contractIBCPortID, msg)
	}

	// Parse and validate the input
	var customMsg types.Msg
	if err := json.Unmarshal(msg.Custom, &customMsg); err != nil {
		return nil, nil, sdkErrors.Wrap(sdkErrors.ErrInvalidRequest, fmt.Sprintf("custom msg JSON unmarshal: %v", err))
	}
	if err := customMsg.Validate(); err != nil {
		return nil, nil, sdkErrors.Wrap(sdkErrors.ErrInvalidRequest, fmt.Sprintf("custom msg validation: %v", err))
	}

	// Execute custom sub-msg (one of)
	switch {
	case customMsg.Claim != nil:
		return e.claimHandler.DispatchMsg(ctx, contractAddr, contractIBCPortID, *customMsg.Claim)
	default:
		// That should never happen, since we validate the input above
		return nil, nil, sdkErrors.Wrap(wasmdTypes.ErrUnknownMsg, "no custom handler found")
	}
}