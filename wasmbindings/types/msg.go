package types

import (
	"fmt"

	claimTypes "github.com/envadiv/Passage3D/wasmbindings/claim/types"
)

// Msg is a container for custom WASM messages
type Msg struct {
	//Claim defines the x/claim module specific sub-message.
	Claim *claimTypes.Msg `json:"claims,omitempty"`
}

// Validate validates the msg fields.
func (m Msg) Validate() error {
	cnt := 0

	if m.Claim != nil {
		cnt++
	}

	if cnt != 1 {
		return fmt.Errorf("one and only one sub-message must be set")
	}

	return nil
}