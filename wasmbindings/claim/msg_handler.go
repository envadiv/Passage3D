package claim

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/envadiv/Passage3D/wasmbindings/claim/types"
	
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
func NewClaimMsgHandler(ck KeeperWriterExpected) MsgHandler {
	return MsgHandler{
		claimsKeeper: ck,
	}
}

// DispatchMsg validates and executes a custom WASM msg.
func (h MsgHandler) DispatchMsg(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg types.Msg) ([]sdk.Event, [][]byte, error) {
	// Validate the input
	if err := msg.Validate(); err != nil {
		return nil, nil, sdkErrors.Wrap(sdkErrors.ErrInvalidRequest, fmt.Sprintf("x/claim: sub-msg validation: %v", err))
	}

	// Execute operation (one of)
	switch {
	case msg.ClaimDrop != nil:
		return h.claimCoins(ctx, contractAddr, *msg.ClaimDrop)
	default:
		return nil, nil, sdkErrors.Wrap(sdkErrors.ErrInvalidRequest, fmt.Sprintf("x/claim: unknown operation"))
	}
}

// updateContractMetadata updates the contract metadata.
func (h MsgHandler) claimCoins(ctx sdk.Context, contractAddr sdk.AccAddress, req types.ClaimRequest) ([]sdk.Event, [][]byte, error) {

	var claimable sdk.Coins
	var err error

	
	claimable, err := h.claimsKeeper.ClaimCoinsForAction(ctx, contractAddr, req.Action)

	resCl, err := json.Marshal(types.NewClaimResponse(claimable))
	if err != nil {
		return nil, nil, fmt.Errorf("result JSON marshal: %w", err)
	}

	return nil, [][]byte{resCl}, nil
}

