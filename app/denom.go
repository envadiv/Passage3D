package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BondDenom is Passage's staking, fee and mint denomination.
const BondDenom = "upasg"

func init() {
	// Module default genesis states (staking bond denom, mint denom, gov deposits,
	// crisis constant fee) are built from sdk.DefaultBondDenom. Making it upasg
	// lets `passage init` / `passage testnet` produce a Passage-denominated
	// genesis instead of "stake". It has no effect on an existing chain.
	sdk.DefaultBondDenom = BondDenom
}
