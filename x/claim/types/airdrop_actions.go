package types

const (
	// ActionInitialClaim defines a  initial claim actions for airdrop.
	ActionInitialClaim int32 = 0
	// DelegateActionStake defines Delegate Action Stake
	InitialClaim = "ActionInitialClaim"
)

var ActionName = map[int32]string{
	0: "ActionInitialClaim",
}

var ActionValue = map[string]int32{
	"ActionInitialClaim": 0,
}
