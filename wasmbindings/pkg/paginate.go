package pkg

import (
	"github.com/cosmos/cosmos-sdk/types/query"
)

// WASM binding of query.PageRequest.
type PageRequest struct {
	// Key is a value returned in the PageResponse.NextKey
	Key []byte `json:"key"`
	// Only one of (Offset, Key) should be set
	Offset uint64 `json:"offset"`
	// Total number of results to be returned
	Limit uint64 `json:"limit"`
	// Ignored when Key field is set
	CountTotal bool `json:"count_total"`
	// Reverse if set to true
	Reverse bool `json:"reverse"`
}

// WASM binding of query.PageResponse.
type PageResponse struct {
	// NextKey is the key to query the next page
	NextKey []byte `json:"next_key"`
	// Total number of results
	Total uint64 `json:"total"`
}

// Converts the SDK version of query.PageRequest to the WASM bindings version.
func NewPageRequestFromSDK(pageReq query.PageRequest) PageRequest {
	return PageRequest{
		Key:        pageReq.Key,
		Offset:     pageReq.Offset,
		Limit:      pageReq.Limit,
		CountTotal: pageReq.CountTotal,
		Reverse:    pageReq.Reverse,
	}
}

// Converts the WASM bindings version of the query.PageResponse to the SDK version.
func (r PageRequest) ToSDK() query.PageRequest {
	return query.PageRequest{
		Key:        r.Key,
		Offset:     r.Offset,
		Limit:      r.Limit,
		CountTotal: r.CountTotal,
		Reverse:    r.Reverse,
	}
}

// Converts the SDK version of the query.PageResponse to the WASM bindings version.
func NewPageResponseFromSDK(pageResp query.PageResponse) PageResponse {
	return PageResponse{
		NextKey: pageResp.NextKey,
		Total:   pageResp.Total,
	}
}

// Converts the WASM bindings version of the query.PageResponse to the SDK version.
func (r PageResponse) ToSDK() query.PageResponse {
	return query.PageResponse{
		NextKey: r.NextKey,
		Total:   r.Total,
	}
}