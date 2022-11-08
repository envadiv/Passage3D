package types

import (
	"fmt"
	"time"

	wasmdTypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmVmTypes "github.com/CosmWasm/wasmvm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/envadiv/Passage3D/wasmbindings/pkg"
	claimTypes "github.com/envadiv/Passage3D/x/claim/types"
)

// ClaimRecordRequest is the Query.ClaimRecord request.
type ClaimRecordRequest struct {
	
	ClaimAddress string `json:"claim_address"`

	ClaimableAmount []types.Coin `"claimable_amount"`

	Paginate *pkg.PageRequest `json:"paginate"`
}

type (
	// ClaimRecordResponse is the Query.ClaimRecord response.
	ClaimRecordResponse struct {
		// Records is the list of claim records returned by the query.
		Records []ClaimRecord `json:"records"`
		// Paginate is the pagination details in the response.
		Paginate pkg.PageResponse `json:"paginate"`
	}

	// ClaimRecord is the WASM binding representation of a claimTypes.ClaimRecord object.
	ClaimRecord struct {
		// address of claim user
		Address string `json:"claim_address"`
		// claimable amount for claim actions
		ClaimableAmount []types.Coin `json:"claimable_amount"`
		
	}
)

// Validate performs request fields validation.
func (r ClaimRecordRequest) Validate() error {
	if _, err := sdk.AccAddressFromBech32(r.Address); err != nil {
		return fmt.Errorf("claimAddress: parsing: %w", err)
	}

	return nil
}

// MustGetClaimAddress returns the claimer address as sdk.AccAddress.
// CONTRACT: panics in case of an error (should not happen since we validate the request).
func (r ClaimRecordRequest) MustGetClaimAddress() sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(r.Address)
	if err != nil {
		// Should not happen since we validate the request before this call
		panic(fmt.Errorf("wasm bindings: claimRecordsRequest request: parsing claimAddress: %w", err))
	}

	return addr
}

// ToSDK converts the ClaimRecord to claimTypes.ClaimRecord.
func (r ClaimRecord) ToSDK() (claimTypes.ClaimRecord, error) {
	claimable, err := pkg.WasmCoinsToSDK(r.ClaimableAmount)
	if err != nil {
		return claimTypes.ClaimRecord{}, fmt.Errorf("claims: %w", err)
	}

	return claimTypes.ClaimRecord{
		Addr:               r.Address,
		Claimable:   r.ClaimableAmount,
		
	}, nil
}


func NewClaimRecordResponse(records []claimTypes.ClaimRecord, pageResp query.PageResponse) ClaimRecordResponse {
	resp := ClaimRecordResponse{
		ClaimAddress:    make([]ClaimRecord, 0, len(records)),
		Paginate: pkg.NewPageResponseFromSDK(pageResp),
	}

	for _, record := range records {
		resp.Records = append(resp.Records, ClaimRecord{
			Addr:               record.Address,
			ClaimableAmount: record.Claimable,
		})
	}

	return resp
}