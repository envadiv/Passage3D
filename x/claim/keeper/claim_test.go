package keeper_test

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/envadiv/Passage3D/x/claim/types"
	"time"
)

func (suite *KeeperTestSuite) TestHookOfUnclaimableAccount() {
	pub1 := secp256k1.GenPrivKey().PubKey()
	addr1 := sdk.AccAddress(pub1.Address())
	suite.app.AccountKeeper.SetAccount(suite.ctx, authtypes.NewBaseAccount(addr1, nil, 0, 0))

	claim, err := suite.app.ClaimKeeper.GetClaimRecord(suite.ctx, addr1)
	suite.NoError(err)
	suite.Equal(types.ClaimRecord{}, claim)

	suite.app.ClaimKeeper.AfterDelegationModified(suite.ctx, addr1, nil)

	balances := suite.app.BankKeeper.GetAllBalances(suite.ctx, addr1)
	suite.Equal(sdk.Coins{}, balances)
}

func (suite *KeeperTestSuite) TestHookBeforeAirdropStart() {
	airdropStartTime := time.Now().Add(time.Hour)

	suite.app.ClaimKeeper.SetParams(suite.ctx, types.Params{
		AirdropEnabled:     true,
		ClaimDenom:         types.DefaultClaimDenom,
		AirdropStartTime:   airdropStartTime,
		DurationUntilDecay: time.Hour,
		DurationOfDecay:    time.Hour * 4,
	})

	pub1 := secp256k1.GenPrivKey().PubKey()
	addr1 := sdk.AccAddress(pub1.Address())

	claimRecords := []types.ClaimRecord{
		{
			Address: addr1.String(),
			ClaimableAmount: []sdk.Coin{
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
			},
			ActionCompleted: []bool{false, false, false},
		},
	}
	suite.app.AccountKeeper.SetAccount(suite.ctx, authtypes.NewBaseAccount(addr1, nil, 0, 0))

	err := suite.app.ClaimKeeper.SetClaimRecords(suite.ctx, claimRecords)
	suite.Require().NoError(err)

	_, err = suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr1)
	suite.NoError(err)
	// Now, it is before starting air drop, so this value should return the empty coins
	//suite.True(coins.Empty())

	_, err = suite.app.ClaimKeeper.GetClaimableAmountForAction(suite.ctx, addr1, types.ActionDelegateStake)
	suite.NoError(err)
	// Now, it is before starting air drop, so this value should return the empty coins
	//suite.True(coins.Empty())

	//suite.app.ClaimKeeper.AfterDelegationModified(suite.ctx, addr1, nil)
	balances := suite.app.BankKeeper.GetAllBalances(suite.ctx, addr1)
	// Now, it is before starting air drop, so claim module should not send the balances to the user after swap.
	suite.True(balances.Empty())

	suite.app.ClaimKeeper.AfterDelegationModified(suite.ctx.WithBlockTime(airdropStartTime), addr1, nil)
	balances = suite.app.BankKeeper.GetAllBalances(suite.ctx, addr1)
	// Now, it is the time for air drop, so claim module should send the balances to the user after swap.
	suite.Equal(balances.String(), claimRecords[0].ClaimableAmount[1].String())
}

func (suite *KeeperTestSuite) TestHookAfterAirdropEnd() {

	// airdrop recipient address
	pub1 := secp256k1.GenPrivKey().PubKey()
	addr1 := sdk.AccAddress(pub1.Address())

	claimRecords := []types.ClaimRecord{
		{
			Address: addr1.String(),
			ClaimableAmount: []sdk.Coin{
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
			},
			ActionCompleted: []bool{false, false, false},
		},
	}
	suite.app.AccountKeeper.SetAccount(suite.ctx, authtypes.NewBaseAccount(addr1, nil, 0, 0))
	err := suite.app.ClaimKeeper.SetClaimRecords(suite.ctx, claimRecords)
	suite.Require().NoError(err)

	params := suite.app.ClaimKeeper.GetParams(suite.ctx)
	suite.ctx = suite.ctx.WithBlockTime(params.AirdropStartTime.Add(params.DurationUntilDecay).Add(params.DurationOfDecay))

	suite.app.ClaimKeeper.EndAirdrop(suite.ctx)

	suite.Require().NotPanics(func() {
		suite.app.ClaimKeeper.AfterDelegationModified(suite.ctx, addr1, sdk.ValAddress(addr1))
	})
}

func (suite *KeeperTestSuite) TestDuplicatedActionNotWithdrawRepeatedly() {
	pub1 := secp256k1.GenPrivKey().PubKey()
	addr1 := sdk.AccAddress(pub1.Address())

	claimRecords := []types.ClaimRecord{
		{
			Address: addr1.String(),
			ClaimableAmount: []sdk.Coin{
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
			},
			ActionCompleted: []bool{false, false, false},
		},
	}
	suite.app.AccountKeeper.SetAccount(suite.ctx, authtypes.NewBaseAccount(addr1, nil, 0, 0))

	err := suite.app.ClaimKeeper.SetClaimRecords(suite.ctx, claimRecords)
	suite.Require().NoError(err)

	coins1, err := suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr1)
	suite.Require().NoError(err)
	suite.Require().True(coins1.IsAllGT(sdk.NewCoins(claimRecords[0].ClaimableAmount[0])))

	suite.app.ClaimKeeper.AfterDelegationModified(suite.ctx, addr1, sdk.ValAddress(addr1))
	claim, err := suite.app.ClaimKeeper.GetClaimRecord(suite.ctx, addr1)
	suite.NoError(err)
	suite.True(claim.ActionCompleted[types.ActionDelegateStake])
	claimedCoins := suite.app.BankKeeper.GetAllBalances(suite.ctx, addr1)
	suite.Require().Equal(claimedCoins.String(), claimRecords[0].ClaimableAmount[1].String())

	suite.app.ClaimKeeper.AfterDelegationModified(suite.ctx, addr1, sdk.ValAddress(addr1))
	claim, err = suite.app.ClaimKeeper.GetClaimRecord(suite.ctx, addr1)
	suite.NoError(err)
	suite.True(claim.ActionCompleted[types.ActionDelegateStake])
	claimedCoins = suite.app.BankKeeper.GetAllBalances(suite.ctx, addr1)
	suite.Require().Equal(claimedCoins.String(), claimRecords[0].ClaimableAmount[1].String())
}

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
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
			},
			ActionCompleted: []bool{false, false, false},
		},
		{
			Address: addr2.String(),
			ClaimableAmount: []sdk.Coin{
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
			},
			ActionCompleted: []bool{false, false, false},
		},
		{
			Address: addr4.String(),
			ClaimableAmount: []sdk.Coin{
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
				sdk.NewInt64Coin(types.DefaultClaimDenom, 1000),
			},
			ActionCompleted: []bool{false, false, false},
		},
	}

	err := suite.app.ClaimKeeper.SetClaimRecords(suite.ctx, claimRecords)
	suite.Require().NoError(err)

	coins1, err := suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr1)
	suite.Require().NoError(err)
	suite.Require().True(coins1.IsAllGT(sdk.NewCoins(claimRecords[0].ClaimableAmount[0])))

	coins2, err := suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr2)
	suite.Require().NoError(err)
	suite.Require().True(coins2.IsAllGT(sdk.NewCoins(claimRecords[1].ClaimableAmount[0])))

	coins3, err := suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr3)
	suite.Require().NoError(err)
	suite.Require().Equal(coins3, sdk.Coins{})

	_, err = suite.app.ClaimKeeper.ClaimCoinsForAction(suite.ctx, addr4, types.ActionInitialClaim)
	suite.Require().NoError(err)
	// get balance after rest actions done
	coins1 = suite.app.BankKeeper.GetAllBalances(suite.ctx, addr4)
	suite.Require().Equal(coins1.String(), sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 1000)).String())

	// get rewards amount per action
	coins4, err := suite.app.ClaimKeeper.GetClaimableAmountForAction(suite.ctx, addr1, types.ActionDelegateStake)
	suite.Require().NoError(err)
	suite.Require().Equal(coins4.String(), sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 1000)).String())

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
	// delegate claim
	suite.app.ClaimKeeper.AfterDelegationModified(suite.ctx, addr1, sdk.ValAddress(addr1))
	// withdraw remaining token action
	action, err = suite.app.ClaimKeeper.ClaimCoinsForAction(suite.ctx, addr1, types.ActionForRemainingAirdrop)
	suite.Require().NoError(err)
	suite.Require().Equal(action.String(), claimRecords[0].ClaimableAmount[2].String())
	// get balance after rest actions done
	coins1 = suite.app.BankKeeper.GetAllBalances(suite.ctx, addr1)
	suite.Require().Equal(coins1.String(), sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 3000)).String())

	// after actions module account balance will decrease (initial : 10000000) ->  (9998000)
	// 3000 for addr1 all_actions  and 1000 for addr4 initial_claim_action
	moduleAccAddr := suite.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	coins := suite.app.BankKeeper.GetBalance(suite.ctx, moduleAccAddr, types.DefaultClaimDenom)
	suite.Require().Equal(coins.String(), sdk.NewInt64Coin(types.DefaultClaimDenom, 9996000).String())

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
