package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type StakingKeeper interface {
	BondDenom(sdk.Context) string
	IterateAllDelegations(ctx sdk.Context, cb func(stakingtypes.Delegation) bool)
}
