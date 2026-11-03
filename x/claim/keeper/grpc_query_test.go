package keeper_test

import (
	"context"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/envadiv/Passage3D/x/claim/types"
)

func (suite *KeeperTestSuite) TestGrpcQueryParams() {
	resp, err := suite.queryClient.Params(context.Background(), &types.QueryParamsRequest{})
	suite.Require().NoError(err)
	suite.Require().Equal(types.DefaultClaimDenom, resp.GetParams().ClaimDenom)
	suite.Require().True(resp.GetParams().AirdropEnabled)
}

func (suite *KeeperTestSuite) TestGrpcQueryModuleAccountBalance() {
	resp, err := suite.queryClient.ModuleAccountBalance(context.Background(), &types.QueryModuleAccountBalanceRequest{})
	suite.Require().NoError(err)
	suite.Require().Equal(
		sdk.NewCoins(sdk.NewCoin(types.DefaultClaimDenom, sdkmath.NewInt(10000000))).String(),
		resp.ModuleAccountBalance.String(),
	)
}

func (suite *KeeperTestSuite) TestGrpcQueryClaimRecords() {
	grpcClient, k := suite.queryClient, suite.app.ClaimKeeper

	addr1 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	addr2 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	claimRecords := []types.ClaimRecord{
		{
			Address:         addr1.String(),
			ClaimableAmount: sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 100)),
			ActionCompleted: []bool{false},
		},
		{
			Address:         addr2.String(),
			ClaimableAmount: sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 200)),
			ActionCompleted: []bool{false},
		},
	}

	err := k.SetClaimRecords(suite.ctx, claimRecords)
	suite.Require().NoError(err)

	resp, err := grpcClient.ClaimRecord(context.Background(), &types.QueryClaimRecordRequest{Address: addr1.String()})
	suite.Require().NoError(err)
	suite.Require().Equal(claimRecords[0], resp.GetClaimRecord())

	actionResp, err := grpcClient.ClaimableForAction(context.Background(), &types.QueryClaimableForActionRequest{
		Address: addr2.String(),
		Action:  types.InitialClaim,
	})
	suite.Require().NoError(err)
	suite.Require().Equal("200upasg", actionResp.Amount.String())

	totalResp, err := grpcClient.TotalClaimable(context.Background(), &types.QueryTotalClaimableRequest{Address: addr1.String()})
	suite.Require().NoError(err)
	suite.Require().Equal(sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 100)).String(), totalResp.Coins.String())

	// unknown addresses have no claim record
	addr3 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	resp, err = grpcClient.ClaimRecord(context.Background(), &types.QueryClaimRecordRequest{Address: addr3.String()})
	suite.Require().NoError(err)
	suite.Require().Equal(types.ClaimRecord{}, resp.GetClaimRecord())
}

func (suite *KeeperTestSuite) TestGrpcQueryInvalidRequests() {
	ctx := context.Background()

	_, err := suite.queryClient.ClaimRecord(ctx, &types.QueryClaimRecordRequest{Address: "invalid"})
	suite.Require().Error(err)

	addr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, err = suite.queryClient.ClaimableForAction(ctx, &types.QueryClaimableForActionRequest{
		Address: addr.String(),
		Action:  "ActionUnknown",
	})
	suite.Require().ErrorContains(err, "invalid action type")

	_, err = suite.queryClient.TotalClaimable(ctx, &types.QueryTotalClaimableRequest{Address: "invalid"})
	suite.Require().Error(err)
}
