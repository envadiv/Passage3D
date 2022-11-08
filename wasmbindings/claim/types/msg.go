package types

import (
	"fmt"
)

// Msg is a container for custom WASM message operations for the x/claim
type Msg struct {
	
	ClaimDrop *ClaimRequest `json:"claim"`
}

// Validate validates the msg fields.
func (m Msg) Validate() error {
	cnt := 0

	if m.ClaimDrop != nil {
		if err := m.ClaimDrop.Validate(); err != nil {
			return fmt.Errorf("ClaimDrop: %w", err)
		}
		cnt++
	}

	if cnt != 1 {
		return fmt.Errorf("one and only one field must be set")
	}

	return nil
}