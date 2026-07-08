package types

import (
	"context"
)

type StakingKeeper interface {
	BondDenom(context.Context) (string, error)
}
