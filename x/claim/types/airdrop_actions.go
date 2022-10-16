package types

const (
	// ActionInitialClaim defines a  initial claim actions for airdrop.
	ActionInitialClaim int32 = 0
	// DelegateActionStake defines Delegate Action Stake
	InitialClaim = "ActionInitialClaim"
)

var Action_name = map[int32]string{ //nolint:revive
	0: "ActionInitialClaim",
}

var Action_value = map[string]int32{ //nolint:revive
	"ActionInitialClaim": 0,
}
