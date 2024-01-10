package keeper_test

import (
	"fmt"

	"github.com/envadiv/Passage3D/x/claim/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) TestAirdropFlow() {
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

	err := s.app.ClaimKeeper.SetClaimRecords(s.ctx, claimRecords)
	s.Require().NoError(err)

	coins1, err := s.app.ClaimKeeper.GetUserTotalClaimable(s.ctx, addr1)
	s.Require().NoError(err)
	s.Require().True(coins1.IsAllGTE(sdk.NewCoins(claimRecords[0].ClaimableAmount[0])))

	coins2, err := s.app.ClaimKeeper.GetUserTotalClaimable(s.ctx, addr2)
	s.Require().NoError(err)
	s.Require().True(coins2.IsAllGTE(sdk.NewCoins(claimRecords[1].ClaimableAmount[0])))

	coins3, err := s.app.ClaimKeeper.GetUserTotalClaimable(s.ctx, addr3)
	s.Require().NoError(err)
	s.Require().Equal(coins3, sdk.Coins{})

	_, err = s.app.ClaimKeeper.ClaimCoinsForAction(s.ctx, addr4, types.ActionInitialClaim)
	s.Require().NoError(err)
	// get balance after rest actions done
	coins1 = s.app.BankKeeper.GetAllBalances(s.ctx, addr4)
	s.Require().Equal(coins1.String(), sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 1000)).String())

	// get completed activities
	claimRecord, err := s.app.ClaimKeeper.GetClaimRecord(s.ctx, addr1)
	s.Require().NoError(err)
	for i := range types.ActionName {
		s.Require().False(claimRecord.ActionCompleted[i])
	}

	// do actions
	// initial claim
	action, err := s.app.ClaimKeeper.ClaimCoinsForAction(s.ctx, addr1, types.ActionInitialClaim)
	s.Require().NoError(err)
	s.Require().Equal(action.String(), claimRecords[0].ClaimableAmount[0].String())
	// get balance after rest actions done
	coins1 = s.app.BankKeeper.GetAllBalances(s.ctx, addr1)
	s.Require().Equal(coins1.String(), sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 1000)).String())

	// after actions module account balance will decrease (initial : 10000000) ->  (9998000)
	moduleAccAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	coins := s.app.BankKeeper.GetBalance(s.ctx, moduleAccAddr, types.DefaultClaimDenom)
	s.Require().Equal(coins.String(), sdk.NewInt64Coin(types.DefaultClaimDenom, 9998000).String())

	// get claimable after withdrawing all
	coins1, err = s.app.ClaimKeeper.GetUserTotalClaimable(s.ctx, addr1)
	s.Require().NoError(err)
	fmt.Println(coins1.String())
	s.Require().True(coins1.Empty())

	err = s.app.ClaimKeeper.EndAirdrop(s.ctx)
	s.Require().NoError(err)

	// after airdrop end all module account balance move to community pool account
	moduleAccAddr = s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	coins = s.app.BankKeeper.GetBalance(s.ctx, moduleAccAddr, types.DefaultClaimDenom)
	s.Require().Equal(coins.String(), sdk.NewInt64Coin(types.DefaultClaimDenom, 0).String())

	coins2, err = s.app.ClaimKeeper.GetUserTotalClaimable(s.ctx, addr2)
	s.Require().NoError(err)
	s.Require().Equal(coins2, sdk.Coins{})
}
