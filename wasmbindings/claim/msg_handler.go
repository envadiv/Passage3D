package claim

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/envadiv/Passage3D/wasmbindings/claim/types"
	claimTypes "github.com/envadiv/Passage3D/x/claim/types"
)

// KeeperWriterExpected defines the x/claim keeper expected write operations.
type KeeperWriterExpected interface {

	ClaimCoinsForAction(ctx sdk.Context, addr sdk.AccAddress, action int32) (sdk.Coin, error) 

}

// MsgHandler provides a custom WASM message handler for the x/claim module.
type MsgHandler struct {
	claimsKeeper KeeperWriterExpected
}

// NewClaimMsgHandler creates a new MsgHandler instance.
func NewClaimMsgHandler(rk KeeperWriterExpected) MsgHandler {
	return MsgHandler{
		claimsKeeper: ck,
	}
}

// DispatchMsg validates and executes a custom WASM msg.
func (h MsgHandler) DispatchMsg(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg types.Msg) ([]sdk.Event, [][]byte, error) {
	// Validate the input
	if err := msg.Validate(); err != nil {
		return nil, nil, sdkErrors.Wrap(claimTypes.ErrInvalidRequest, fmt.Sprintf("x/claim: sub-msg validation: %v", err))
	}

	// Execute operation (one of)
	switch {
	case msg.ClaimDrop != nil:
		return h.claimCoins(ctx, contractAddr, *msg.ClaimDrop)
	default:
		return nil, nil, sdkErrors.Wrap(claimTypes.ErrInvalidRequest, "x/claim: unknown operation")
	}
}

// updateContractMetadata updates the contract metadata.
func (h MsgHandler) claimCoins(ctx sdk.Context, contractAddr sdk.AccAddress, req types.ClaimRequest) ([]sdk.Event, sdk.Coins, error) {

	var claimable sdk.Coins

	if req.Action != nil {
		claimable, err = h.claimsKeeper.ClaimCoinsForAction(ctx sdk.Context, addr sdk.AccAddress, req.Action)
	}

	if err != nil {
		return nil, nil, err
	}

	return nil, claimable, nil
}

