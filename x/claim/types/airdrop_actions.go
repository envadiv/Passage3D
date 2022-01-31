package types

type Action int32

const (
	// ActionDelegateStake defines a delegate stake actions for airdrop.
	ActionDelegateStake Action = 0
	// TODO: We need to add more actions
)

var Action_name = map[int32]string{
	0: "ActionDelegateStake",
}

var Action_value = map[string]int32{
	"ActionDelegateStake": 0,
}
