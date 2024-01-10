package keeper_test

import (
	"testing"
	"time"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/x/claim/keeper"
	"github.com/envadiv/Passage3D/x/claim/types"
	"github.com/stretchr/testify/suite"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type KeeperTestSuite struct {
	suite.Suite

	ctx         sdk.Context
	queryClient types.QueryClient
	app         *app.PassageApp
}

func (s *KeeperTestSuite) SetupTest() {
	s.app = app.Setup(false)
	s.ctx = s.app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "passage3d-1", Time: time.Now()})

	airdropStartTime := time.Now()
	s.app.ClaimKeeper.CreateModuleAccount(s.ctx, sdk.NewCoin(types.DefaultClaimDenom, sdk.NewInt(10000000)))

	s.app.ClaimKeeper.SetParams(s.ctx, types.Params{
		AirdropEnabled:     true,
		AirdropStartTime:   airdropStartTime,
		DurationUntilDecay: types.DefaultDurationUntilDecay,
		DurationOfDecay:    types.DefaultDurationOfDecay,
		ClaimDenom:         types.DefaultClaimDenom,
	})

	querier := keeper.Querier{Keeper: s.app.ClaimKeeper}

	queryHelper := baseapp.NewQueryServerTestHelper(s.ctx, s.app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, querier)
	s.queryClient = types.NewQueryClient(queryHelper)

	s.ctx = s.ctx.WithBlockTime(airdropStartTime)
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}
