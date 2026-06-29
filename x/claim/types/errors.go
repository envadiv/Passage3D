package types

// DONTCOVER

import (
	sdkerrors "cosmossdk.io/errors"
)

// x/claim module sentinel errors
var (
	ErrAirdropNotEnabled             = sdkerrors.Register(ModuleName, 2, "airdrop not enabled")
	ErrIncorrectModuleAccountBalance = sdkerrors.Register(ModuleName, 3, "claim module account balance != sum of all claim record InitialClaimableAmounts")
)
