package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/x/claim/types"
	"github.com/stretchr/testify/suite"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"testing"
	"time"
)

type KeeperTestSuite struct {
	suite.Suite

	ctx sdk.Context
	// querier sdk.Querier
	app *app.PassageApp
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.app = app.Setup(false)
	suite.ctx = suite.app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "passage3d-1", Time: time.Now().UTC()})

	airdropStartTime := time.Now()
	suite.app.ClaimKeeper.CreateModuleAccount(suite.ctx, sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10000000)))

	suite.app.ClaimKeeper.SetParams(suite.ctx, types.Params{
		AirdropEnabled:     true,
		AirdropStartTime:   airdropStartTime,
		DurationUntilDecay: types.DefaultDurationUntilDecay,
		DurationOfDecay:    types.DefaultDurationOfDecay,
		ClaimDenom:         sdk.DefaultBondDenom,
	})

	suite.ctx = suite.ctx.WithBlockTime(airdropStartTime)
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}
