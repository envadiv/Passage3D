package types

const (
	// ActionInitialClaim defines a  initial claim actions for airdrop.
	ActionInitialClaim int32 = 0
	// ActionDelegateStake defines a delegate stake actions for airdrop.
	ActionDelegateStake int32 = 1
	// ActionForRemainingAirdrop defines the action for remaining tokens
	ActionVote int32 = 2

	// DelegateActionStake defines Delegate Action Stake
	DelegateActionStake = "ActionDelegateStake"
)

var Action_name = map[int32]string{
	0: "ActionInitialClaim",
	1: "ActionDelegateStake",
	2: "ActionVote",
}

var Action_value = map[string]int32{
	"ActionInitialClaim":  0,
	"ActionDelegateStake": 1,
	"ActionVote":          2,
}
