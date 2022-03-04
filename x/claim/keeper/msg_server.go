package keeper

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/envadiv/Passage3D/x/claim/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// Claim sends the claim amount to user based on action type from airdrop
func (k msgServer) Claim(goCtx context.Context, msg *types.MsgClaim) (*types.MsgClaimResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, err
	}
	params := k.GetParams(ctx)
	if !params.IsAirdropEnabled(ctx.BlockTime()) {
		return nil, types.ErrAirdropNotEnabled
	}
	claimableCoinsForAction, err := k.Keeper.ClaimCoinsForAction(ctx, sender, types.Action_value[msg.ClaimAction])
	if err != nil {
		return nil, err
	}
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeyAction, msg.ClaimAction),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender),
		),
	})
	return &types.MsgClaimResponse{
		ClaimedAmount: claimableCoinsForAction,
	}, nil
}
