package keeper_test

import (
	sdkmath "cosmossdk.io/math"
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/x/claim/keeper"
	"github.com/envadiv/Passage3D/x/claim/types"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type KeeperTestSuite struct {
	suite.Suite

	ctx         sdk.Context
	queryClient types.QueryClient
	app         *app.PassageApp
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.app = app.Setup(false, suite.T())
	suite.ctx = suite.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "passage3d-1", Time: time.Now()})

	airdropStartTime := time.Now()
	suite.app.ClaimKeeper.CreateModuleAccount(suite.ctx, sdk.NewCoin(types.DefaultClaimDenom, sdkmath.NewInt(10000000)))

	suite.app.ClaimKeeper.SetParams(suite.ctx, types.Params{
		AirdropEnabled:     true,
		AirdropStartTime:   airdropStartTime,
		DurationUntilDecay: types.DefaultDurationUntilDecay,
		DurationOfDecay:    types.DefaultDurationOfDecay,
		ClaimDenom:         types.DefaultClaimDenom,
	})

	querier := keeper.Querier{Keeper: suite.app.ClaimKeeper}

	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, suite.app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, querier)
	suite.queryClient = types.NewQueryClient(queryHelper)

	suite.ctx = suite.ctx.WithBlockTime(airdropStartTime)
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}
