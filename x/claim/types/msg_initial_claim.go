package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgClaim{}

// msg types
const (
	TypeMsgClaim = "claim"
)

func NewMsgClaim(sender, action string) *MsgClaim {
	return &MsgClaim{
		Sender:      sender,
		ClaimAction: action,
	}
}

func (MsgClaim) Route() string {
	return RouterKey
}

func (MsgClaim) Type() string {
	return TypeMsgClaim
}

func (msg MsgClaim) GetSigners() []sdk.AccAddress {
	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{sender}
}

func (msg MsgClaim) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(&msg)
	return sdk.MustSortJSON(bz)
}

func (msg MsgClaim) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address (%s)", err)
	}
	if len(msg.ClaimAction) == 0 {
		return sdkerrors.Wrapf(sdkerrors.ErrNotFound, "empty action, action type is required")
	}
	return nil
}
