package keeper_test

import (
	"fmt"

	"github.com/envadiv/Passage3D/x/claim/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperTestSuite) TestAirdropFlow() {
	addr1 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	addr2 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	addr3 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	addr4 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	claimRecords := []types.ClaimRecord{
		{
			Address: addr1.String(),
			ClaimableAmount: []sdk.Coin{
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
			},
			ActionCompleted: []bool{false},
		},
		{
			Address: addr2.String(),
			ClaimableAmount: []sdk.Coin{
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
			},
			ActionCompleted: []bool{false},
		},
		{
			Address: addr4.String(),
			ClaimableAmount: []sdk.Coin{
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
			},
			ActionCompleted: []bool{false},
		},
	}

	err := suite.app.ClaimKeeper.SetClaimRecords(suite.ctx, claimRecords)
	suite.Require().NoError(err)

	coins1, err := suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr1)
	suite.Require().NoError(err)
	suite.Require().True(coins1.IsAllGTE(sdk.NewCoins(claimRecords[0].ClaimableAmount[0])))

	coins2, err := suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr2)
	suite.Require().NoError(err)
	suite.Require().True(coins2.IsAllGTE(sdk.NewCoins(claimRecords[1].ClaimableAmount[0])))

	coins3, err := suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr3)
	suite.Require().NoError(err)
	suite.Require().Equal(coins3, sdk.Coins{})

	_, err = suite.app.ClaimKeeper.ClaimCoinsForAction(suite.ctx, addr4, types.ActionInitialClaim)
	suite.Require().NoError(err)
	// get balance after rest actions done
	coins1 = suite.app.BankKeeper.GetAllBalances(suite.ctx, addr4)
	suite.Require().Equal(coins1.String(), sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 1000)).String())

	// get completed activities
	claimRecord, err := suite.app.ClaimKeeper.GetClaimRecord(suite.ctx, addr1)
	suite.Require().NoError(err)
	for i := range types.Action_name {
		suite.Require().False(claimRecord.ActionCompleted[i])
	}

	// do actions
	// initial claim
	action, err := suite.app.ClaimKeeper.ClaimCoinsForAction(suite.ctx, addr1, types.ActionInitialClaim)
	suite.Require().NoError(err)
	suite.Require().Equal(action.String(), claimRecords[0].ClaimableAmount[0].String())
	// get balance after rest actions done
	coins1 = suite.app.BankKeeper.GetAllBalances(suite.ctx, addr1)
	suite.Require().Equal(coins1.String(), sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 1000)).String())

	// after actions module account balance will decrease (initial : 10000000) ->  (9998000)
	moduleAccAddr := suite.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	coins := suite.app.BankKeeper.GetBalance(suite.ctx, moduleAccAddr, types.DefaultClaimDenom)
	suite.Require().Equal(coins.String(), sdk.NewInt64Coin(types.DefaultClaimDenom, 9998000).String())

	// get claimable after withdrawing all
	coins1, err = suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr1)
	suite.Require().NoError(err)
	fmt.Println(coins1.String())
	suite.Require().True(coins1.Empty())

	err = suite.app.ClaimKeeper.EndAirdrop(suite.ctx)
	suite.Require().NoError(err)

	// after airdrop end all module account balance move to community pool account
	moduleAccAddr = suite.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	coins = suite.app.BankKeeper.GetBalance(suite.ctx, moduleAccAddr, types.DefaultClaimDenom)
	suite.Require().Equal(coins.String(), sdk.NewInt64Coin(types.DefaultClaimDenom, 0).String())

	coins2, err = suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr2)
	suite.Require().NoError(err)
	suite.Require().Equal(coins2, sdk.Coins{})
}
