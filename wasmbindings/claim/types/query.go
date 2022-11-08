package types

import (
	"fmt"
)

// Query is a container for custom WASM query for the x/claim module.
type Query struct {
	ClaimRecord *ClaimRecordRequest `json:"claim_records"`
}

// Validate validates the query fields.
func (q Query) Validate() error {
	cnt := 0

	if q.ClaimRecord != nil {
		if err := q.ClaimRecord.Validate(); err != nil {
			return fmt.Errorf("claimRecord: %w", err)
		}
		cnt++
	}

	if cnt != 1 {
		return fmt.Errorf("one and only one field must be set")
	}

	return nil
}