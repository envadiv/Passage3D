package ante

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

const blockedMultisigAddr = "pasg105488mw9t3qtp62jhllde28v40xqxpjksjqmvx"

// BlockAccountDecorator restricts the community pool multisig account's transactions, except for the community fund.
// Call next AnteHandler if the message is allowed
type BlockAccountDecorator struct{ cdc codec.Codec }

func NewBlockAccountDecorator(cdc codec.Codec) BlockAccountDecorator {
	return BlockAccountDecorator{cdc: cdc}
}

func (bad BlockAccountDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	if simulate {
		return next(ctx, tx, simulate)
	}

	msgs := tx.GetMsgs()
	// handle msg based on type
	if err := handleMessages(bad.cdc, msgs); err != nil {
		return ctx, err
	}

	return next(ctx, tx, simulate)
}

// handleMessages check and handle each msg with rules
func handleMessages(cdc codec.Codec, msgs []sdk.Msg) error {
	for _, msg := range msgs {

		if msgExec, ok := msg.(*authztypes.MsgExec); ok {
			msgs, err := msgExec.GetMessages()
			if err != nil {
				return err
			}

			if err := handleMessages(cdc, msgs); err != nil {
				return err
			}
		} else if _, ok := msg.(*distributiontypes.MsgFundCommunityPool); ok {
			return nil
		}

		signers, _, sgErr := cdc.GetMsgV1Signers(msg)
		if sgErr != nil {
			return sgErr
		}
		for _, signer := range signers {
			if sdk.AccAddress(signer).String() == blockedMultisigAddr {
				return sdkerrors.ErrUnauthorized.Wrapf("%s is not allowed to perform this transaction", blockedMultisigAddr)
			}
		}
	}

	return nil
}
