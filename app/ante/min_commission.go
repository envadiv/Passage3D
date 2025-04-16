package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var MinCommissionRate = sdk.MustNewDecFromStr("0.05")

// ValidateMinCommissionDecorator validates the minimum commission rate of validator
// to be not less than minimum commission rate when creating or editing validator.
type ValidateMinCommissionDecorator struct{}

func NewValidateMinCommissionDecorator() ValidateMinCommissionDecorator {
	return ValidateMinCommissionDecorator{}
}

func (mcd ValidateMinCommissionDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	msgs := tx.GetMsgs()

	// handle msg based on type
	if err := mcd.handleMsgs(msgs); err != nil {
		return ctx, err
	}

	return next(ctx, tx, simulate)
}

func (mcd ValidateMinCommissionDecorator) handleMsgs(msgs []sdk.Msg) error {
	for _, msg := range msgs {
		switch m := msg.(type) {
		case *stakingtypes.MsgCreateValidator:
			if err := validateMinCommissionRate(m.Commission.Rate); err != nil {
				return err
			}

		case *stakingtypes.MsgEditValidator:
			if m.CommissionRate != nil {
				if err := validateMinCommissionRate(*m.CommissionRate); err != nil {
					return err
				}
			}

		case *authztypes.MsgExec:
			execMsgs, err := m.GetMessages()
			if err != nil {
				return err
			}

			if err := mcd.handleMsgs(execMsgs); err != nil {
				return err
			}

		}
	}
	return nil
}

func validateMinCommissionRate(rate sdk.Dec) error {
	if rate.IsNil() || rate.LT(MinCommissionRate) {
		return sdkerrors.ErrInvalidRequest.Wrapf(
			"cannot set validator commission to less than minimum rate of %s", MinCommissionRate)
	}

	return nil
}
