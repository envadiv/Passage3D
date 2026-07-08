package ante

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	BaseDenom          string = "upasg"
	DefaultMinGasPrice string = "12.5"
)

type ValidateMinGasPricesDecorator struct{}

func NewValidateMinGasPricesDecorator() ValidateMinGasPricesDecorator {
	return ValidateMinGasPricesDecorator{}
}

func (mgp ValidateMinGasPricesDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	// Ensure that the min gas prices is not less than default minimum gas price
	if ctx.IsCheckTx() && !simulate {
		minGasPrices := ctx.MinGasPrices()
		if err := ValidateMinGasPrices(minGasPrices); err != nil {
			return ctx, err
		}
	}

	return next(ctx, tx, simulate)
}

// ValidateMinGasPrices validates given minimum gas prices
func ValidateMinGasPrices(minGasPrices sdk.DecCoins) error {
	if minGasPrices.AmountOf(BaseDenom).LT(math.LegacyMustNewDecFromStr(DefaultMinGasPrice)) {
		return fmt.Errorf("minimum-gas-prices value should be greater than or equal to %s%s",
			DefaultMinGasPrice, BaseDenom)
	}

	return nil
}
