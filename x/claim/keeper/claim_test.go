package keeper_test

import (
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/envadiv/Passage3D/x/claim/types"
)

func newClaimRecord(addr sdk.AccAddress, amount int64) types.ClaimRecord {
	return types.ClaimRecord{
		Address:         addr.String(),
		ClaimableAmount: []sdk.Coin{sdk.NewInt64Coin(types.DefaultClaimDenom, amount)},
		ActionCompleted: []bool{false},
	}
}

func (suite *KeeperTestSuite) TestAirdropFlow() {
	addr1 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	addr2 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	addr3 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	addr4 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	claimRecords := []types.ClaimRecord{
		newClaimRecord(addr1, 1000),
		newClaimRecord(addr2, 1000),
		newClaimRecord(addr4, 1000),
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
	suite.Require().Equal(sdk.Coins{}, coins3)

	_, err = suite.app.ClaimKeeper.ClaimCoinsForAction(suite.ctx, addr4, types.ActionInitialClaim)
	suite.Require().NoError(err)
	coins4 := suite.app.BankKeeper.GetAllBalances(suite.ctx, addr4)
	suite.Require().Equal(sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 1000)).String(), coins4.String())

	// no action completed yet
	claimRecord, err := suite.app.ClaimKeeper.GetClaimRecord(suite.ctx, addr1)
	suite.Require().NoError(err)
	for i := range types.Action_name {
		suite.Require().False(claimRecord.ActionCompleted[i])
	}

	// initial claim
	claimed, err := suite.app.ClaimKeeper.ClaimCoinsForAction(suite.ctx, addr1, types.ActionInitialClaim)
	suite.Require().NoError(err)
	suite.Require().Equal(claimRecords[0].ClaimableAmount[0].String(), claimed.String())
	coins1 = suite.app.BankKeeper.GetAllBalances(suite.ctx, addr1)
	suite.Require().Equal(sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimDenom, 1000)).String(), coins1.String())

	// claiming the same action again pays nothing
	claimed, err = suite.app.ClaimKeeper.ClaimCoinsForAction(suite.ctx, addr1, types.ActionInitialClaim)
	suite.Require().NoError(err)
	suite.Require().True(claimed.IsNil())
	suite.Require().Equal(coins1.String(), suite.app.BankKeeper.GetAllBalances(suite.ctx, addr1).String())

	// module account balance decreased: 10000000 -> 9998000
	moduleAccAddr := suite.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	coins := suite.app.BankKeeper.GetBalance(suite.ctx, moduleAccAddr, types.DefaultClaimDenom)
	suite.Require().Equal(sdk.NewInt64Coin(types.DefaultClaimDenom, 9998000).String(), coins.String())

	// nothing left to claim for addr1
	coins1, err = suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr1)
	suite.Require().NoError(err)
	suite.Require().True(coins1.Empty())

	feePool, err := suite.app.DistrKeeper.FeePool.Get(suite.ctx)
	suite.Require().NoError(err)
	poolBefore := feePool.CommunityPool

	err = suite.app.ClaimKeeper.EndAirdrop(suite.ctx)
	suite.Require().NoError(err)

	// the whole module account balance moved to the community pool
	coins = suite.app.BankKeeper.GetBalance(suite.ctx, moduleAccAddr, types.DefaultClaimDenom)
	suite.Require().Equal(sdk.NewInt64Coin(types.DefaultClaimDenom, 0).String(), coins.String())
	feePool, err = suite.app.DistrKeeper.FeePool.Get(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(
		poolBefore.Add(sdk.NewDecCoinFromCoin(sdk.NewInt64Coin(types.DefaultClaimDenom, 9998000))).String(),
		feePool.CommunityPool.String(),
	)

	// claim records are gone and the airdrop is disabled
	suite.Require().Empty(suite.app.ClaimKeeper.GetClaimRecords(suite.ctx))
	coins2, err = suite.app.ClaimKeeper.GetUserTotalClaimable(suite.ctx, addr2)
	suite.Require().NoError(err)
	suite.Require().Equal(sdk.Coins{}, coins2)
	suite.Require().False(suite.app.ClaimKeeper.GetParams(suite.ctx).AirdropEnabled)

	_, err = suite.app.ClaimKeeper.ClaimCoinsForAction(suite.ctx, addr2, types.ActionInitialClaim)
	suite.Require().ErrorIs(err, types.ErrAirdropNotEnabled)
}

func (suite *KeeperTestSuite) TestClaimableAmountDecay() {
	addr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	suite.Require().NoError(suite.app.ClaimKeeper.SetClaimRecord(suite.ctx, newClaimRecord(addr, 1000)))

	untilDecay, decay := types.DefaultDurationUntilDecay, types.DefaultDurationOfDecay
	testCases := []struct {
		name    string
		elapsed time.Duration
		want    string // claimable amount, "" when nothing is claimable
	}{
		{"airdrop start", 0, "1000upasg"},
		{"decay start", untilDecay, "1000upasg"},
		{"20% decayed", untilDecay + decay/5, "800upasg"},
		{"half decayed", untilDecay + decay/2, "500upasg"},
		{"decay end", untilDecay + decay, ""},
		{"airdrop over", untilDecay + decay + time.Second, ""},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			ctx := suite.ctx.WithBlockTime(suite.airdropStartTime.Add(tc.elapsed))

			amount, err := suite.app.ClaimKeeper.GetClaimableAmountForAction(ctx, addr, types.ActionInitialClaim)
			suite.Require().NoError(err)
			if tc.want == "" {
				suite.Require().True(amount.IsNil() || amount.IsZero(), amount.String())
			} else {
				suite.Require().Equal(tc.want, amount.String())
			}

			total, err := suite.app.ClaimKeeper.GetUserTotalClaimable(ctx, addr)
			suite.Require().NoError(err)
			suite.Require().Equal(tc.want, total.String())
		})
	}

	// A claim during the decay period pays the decayed amount.
	ctx := suite.ctx.WithBlockTime(suite.airdropStartTime.Add(untilDecay + decay/2))
	claimed, err := suite.app.ClaimKeeper.ClaimCoinsForAction(ctx, addr, types.ActionInitialClaim)
	suite.Require().NoError(err)
	suite.Require().Equal("500upasg", claimed.String())
	suite.Require().Equal("500upasg", suite.app.BankKeeper.GetAllBalances(ctx, addr).String())
}
