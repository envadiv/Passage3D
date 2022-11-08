package types

import (
	"fmt"

	wasmdTypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmVmTypes "github.com/CosmWasm/wasmvm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ClaimRequest is the Msg.Claim request.
type ClaimRequest struct {
	
	// Only one of (RecordsLimit, RecordIDs) should be set.
	Sender string `json:"sender_address"`
	// RecordIDs defines specific ClaimRecord object IDs to process.
	Action string `json:"action"`
}

// ClaimResponse is the Msg.Claim response.
type ClaimResponse struct {
	// RecordsNum is the number of ClaimRecord objects processed by the request.
	Claimable uint64 `json:"claimable"`
	
}

// Validate performs request fields validation.
func (r ClaimRequest) Validate() error {
	if (r.Sender == nil && r.Action == nil) {
		return fmt.Errorf("one of (RecordsLimit, RecordIDs) fields must be set")
	}

	return nil
}

// NewClaimResponse creates a new ClaimResponse.
func NewClaimResponse(recordsUsed int) ClaimResponse {
	return ClaimResponse{
		RecordsNum:   uint64(recordsUsed),
	}
}