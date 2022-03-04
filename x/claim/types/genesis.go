package types

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		ModuleAccountBalance: sdk.NewCoin(DefaultClaimDenom, sdk.ZeroInt()),
		Params: Params{
			AirdropStartTime:   time.Time{},
			DurationUntilDecay: DefaultDurationUntilDecay, // 2 month
			DurationOfDecay:    DefaultDurationOfDecay,    // 4 months
			ClaimDenom:         DefaultClaimDenom,         // upasg
		},
		ClaimRecords: []ClaimRecord{},
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	totalClaimable := sdk.Coins{}

	if err := gs.Params.Validate(); err != nil {
		return err
	}

	for _, claimRecord := range gs.ClaimRecords {
		for _, ClaimableAmount := range claimRecord.ClaimableAmount {
			totalClaimable = totalClaimable.Add(ClaimableAmount)
		}
	}

	if !totalClaimable.IsEqual(sdk.NewCoins(gs.ModuleAccountBalance)) {
		return ErrIncorrectModuleAccountBalance
	}

	if gs.Params.ClaimDenom != gs.ModuleAccountBalance.Denom {
		return fmt.Errorf("denom for module and claim does not match")
	}
	return nil
}
