package v2_5

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

// SetValidatorsMinCommissionRate update the minimum commission rate of the validator
// whose commission rate is below the minimum commission rate.
func SetValidatorsMinCommissionRate(ctx context.Context, sk *staking.Keeper, minCommissionRate math.LegacyDec) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	validators, err := sk.GetAllValidators(ctx)
	if err != nil {
		return err
	}

	for _, validator := range validators {
		if validator.Commission.Rate.IsNil() || validator.Commission.Rate.LT(minCommissionRate) {
			valAddr, err := sdk.ValAddressFromBech32(validator.GetOperator())
			if err != nil {
				return err
			}
			// call before validator modified hooks in staking keeper
			if err := sk.Hooks().BeforeValidatorModified(ctx, valAddr); err != nil {
				return err
			}
			validator.Commission.Rate = minCommissionRate
			validator.Commission.UpdateTime = sdkCtx.BlockTime()
			if err := sk.SetValidator(ctx, validator); err != nil {
				return err
			}
		}
	}

	return nil
}
