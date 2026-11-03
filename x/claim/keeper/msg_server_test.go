package keeper_test

import (
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/envadiv/Passage3D/x/claim/types"
)

func (suite *KeeperTestSuite) TestMsgServerClaim() {
	addr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	suite.Require().NoError(suite.app.ClaimKeeper.SetClaimRecord(suite.ctx, newClaimRecord(addr, 1000)))

	res, err := suite.msgServer.Claim(suite.ctx, types.NewMsgClaim(addr.String(), types.InitialClaim))
	suite.Require().NoError(err)
	suite.Require().Equal("1000upasg", res.ClaimedAmount.String())
	suite.Require().Equal("1000upasg", suite.app.BankKeeper.GetAllBalances(suite.ctx, addr).String())

	record, err := suite.app.ClaimKeeper.GetClaimRecord(suite.ctx, addr)
	suite.Require().NoError(err)
	suite.Require().Equal([]bool{true}, record.ActionCompleted)

	// A completed action cannot be claimed twice.
	res, err = suite.msgServer.Claim(suite.ctx, types.NewMsgClaim(addr.String(), types.InitialClaim))
	suite.Require().NoError(err)
	suite.Require().True(res.ClaimedAmount.IsNil())
	suite.Require().Equal("1000upasg", suite.app.BankKeeper.GetAllBalances(suite.ctx, addr).String())

	// Addresses without a claim record get nothing.
	other := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	res, err = suite.msgServer.Claim(suite.ctx, types.NewMsgClaim(other.String(), types.InitialClaim))
	suite.Require().NoError(err)
	suite.Require().True(res.ClaimedAmount.IsNil())
	suite.Require().True(suite.app.BankKeeper.GetAllBalances(suite.ctx, other).Empty())
}

func (suite *KeeperTestSuite) TestMsgServerClaimErrors() {
	addr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	suite.Require().NoError(suite.app.ClaimKeeper.SetClaimRecord(suite.ctx, newClaimRecord(addr, 1000)))

	_, err := suite.msgServer.Claim(suite.ctx, types.NewMsgClaim("invalid", types.InitialClaim))
	suite.Require().Error(err)

	// Before the airdrop starts.
	ctx := suite.ctx.WithBlockTime(suite.airdropStartTime.Add(-1))
	_, err = suite.msgServer.Claim(ctx, types.NewMsgClaim(addr.String(), types.InitialClaim))
	suite.Require().ErrorIs(err, types.ErrAirdropNotEnabled)

	// Airdrop disabled.
	params := suite.app.ClaimKeeper.GetParams(suite.ctx)
	params.AirdropEnabled = false
	suite.app.ClaimKeeper.SetParams(suite.ctx, params)
	_, err = suite.msgServer.Claim(suite.ctx, types.NewMsgClaim(addr.String(), types.InitialClaim))
	suite.Require().ErrorIs(err, types.ErrAirdropNotEnabled)

	suite.Require().True(suite.app.BankKeeper.GetAllBalances(suite.ctx, addr).Empty())
}
