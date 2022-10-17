package keeper_test

import (
	"context"
	"time"

	"github.com/envadiv/Passage3D/x/claim/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperTestSuite) TestGrpcQueryParams() {
	grpcClient := suite.queryClient

	resp, _ := grpcClient.Params(context.Background(), &types.QueryParamsRequest{})
	suite.Require().Equal(resp.GetParams().ClaimDenom, types.DefaultClaimDenom)
}

func (suite *KeeperTestSuite) TestGrpcQueryModuleAccountBalance() {
	grpcClient := suite.queryClient

	resp, _ := grpcClient.ModuleAccountBalance(context.Background(), &types.QueryModuleAccountBalanceRequest{})
	suite.Require().Equal(resp.ModuleAccountBalance.String(), sdk.NewCoins(sdk.NewCoin(types.DefaultClaimDenom, sdk.NewInt(10000000))).String())
}

func (suite *KeeperTestSuite) TestGrpcQueryClaimRecords() {
	grpcClient, ctx, k := suite.queryClient, suite.ctx, suite.app.ClaimKeeper
	ctx = ctx.WithBlockTime(time.Now())

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

	err := k.SetClaimRecords(ctx, claimRecords)
	suite.Require().NoError(err)

	resp, err := grpcClient.ClaimRecord(context.Background(), &types.QueryClaimRecordRequest{Address: addr1.String()})
	suite.Require().NoError(err)
	suite.Require().Equal(resp.GetClaimRecord(), claimRecords[0])

	//// get claim record for action
	//actionResp, err := grpcClient.TotalClaimable(context.Background(), &types.QueryTotalClaimableRequest{Address: addr1.String()})
	//suite.Require().NoError(err)
	//suite.Require().Equal(actionResp.String(), sdk.NewCoins(sdk.NewCoin(types.DefaultClaimDenom, sdk.NewInt(100))).String())
}
