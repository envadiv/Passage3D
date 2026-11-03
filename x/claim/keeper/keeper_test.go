package keeper_test

import (
	"testing"
	"time"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/x/claim/keeper"
	"github.com/envadiv/Passage3D/x/claim/types"
)

type KeeperTestSuite struct {
	suite.Suite

	ctx              sdk.Context
	queryClient      types.QueryClient
	msgServer        types.MsgServer
	app              *app.PassageApp
	airdropStartTime time.Time
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.app = app.Setup(false, suite.T())
	suite.airdropStartTime = time.Now().UTC()
	// app.Setup commits its first block; write straight to the commit
	// multistore at the next height.
	suite.ctx = suite.app.BaseApp.NewUncachedContext(false, cmtproto.Header{
		Height:  suite.app.LastBlockHeight() + 1,
		ChainID: "passage3d-1",
		Time:    suite.airdropStartTime,
	})

	suite.app.ClaimKeeper.CreateModuleAccount(suite.ctx, sdk.NewCoin(types.DefaultClaimDenom, sdkmath.NewInt(10000000)))

	suite.app.ClaimKeeper.SetParams(suite.ctx, types.Params{
		AirdropEnabled:     true,
		AirdropStartTime:   suite.airdropStartTime,
		DurationUntilDecay: types.DefaultDurationUntilDecay,
		DurationOfDecay:    types.DefaultDurationOfDecay,
		ClaimDenom:         types.DefaultClaimDenom,
	})

	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, suite.app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, keeper.Querier{Keeper: suite.app.ClaimKeeper})
	suite.queryClient = types.NewQueryClient(queryHelper)
	suite.msgServer = keeper.NewMsgServerImpl(suite.app.ClaimKeeper)
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}
