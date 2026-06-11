package v2_5

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

// SetValidatorsMinCommissionRate update the minimum commission rate of the validator
// whose commission rate is below the minimum commission rate.
func SetValidatorsMinCommissionRate(ctx sdk.Context, sk staking.Keeper, minCommissionRate sdk.Dec) error {
	validators := sk.GetAllValidators(ctx)

	for _, validator := range validators {
		if validator.Commission.Rate.IsNil() || validator.Commission.Rate.LT(minCommissionRate) {
			// call before validator modified hooks in staking keeper
			if err := sk.Hooks().BeforeValidatorModified(ctx, validator.GetOperator()); err != nil {
				return err
			}
			validator.Commission.Rate = minCommissionRate
			validator.Commission.UpdateTime = ctx.BlockTime()
			sk.SetValidator(ctx, validator)
		}
	}

	return nil
}
