package types

import (
	"fmt"

	wasmdTypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmVmTypes "github.com/CosmWasm/wasmvm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ClaimRequest is the Msg.Claim request.
type ClaimRequest struct {
	Sender string `json:"sender_address"`
	Action int32 `json:"action"`
}

// ClaimResponse is the Msg.Claim response.
type ClaimResponse struct {
	// RecordsNum is the number of ClaimRecord objects processed by the request.
	Claimable wasmVmTypes.Coin `json:"claimable"`
	
}

// Validate performs request fields validation.
func (r ClaimRequest) Validate() error {
	if (r.Sender == "" && r.Action == 0) {
		return fmt.Errorf("Fields must be set")
	}

	return nil
}

// NewClaimResponse creates a new ClaimResponse.
func NewClaimResponse(claimable sdk.Coin) ClaimResponse {
	return ClaimResponse{
		Claimable:   wasmdTypes.NewWasmCoins(claimable),
	}
}