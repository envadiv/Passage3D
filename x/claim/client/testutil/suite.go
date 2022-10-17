package testutil

import (
	"fmt"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/testutil/network"
	"github.com/envadiv/Passage3D/x/claim/client/cli"
	claimtypes "github.com/envadiv/Passage3D/x/claim/types"
	"github.com/stretchr/testify/suite"
	tmcli "github.com/tendermint/tendermint/libs/cli"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var addr1 sdk.AccAddress
var addr2 sdk.AccAddress

var claimRecords []claimtypes.ClaimRecord

func init() {
	app.SetAddressPrefixes()
	addr1 = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	addr2 = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	claimRecords = []claimtypes.ClaimRecord{
		{
			Address:         addr1.String(),
			ClaimableAmount: sdk.NewCoins(sdk.NewInt64Coin(claimtypes.DefaultClaimDenom, 10)),
			ActionCompleted: []bool{false, false, false, false},
		},
		{
			Address:         addr2.String(),
			ClaimableAmount: sdk.NewCoins(sdk.NewInt64Coin(claimtypes.DefaultClaimDenom, 20)),
			ActionCompleted: []bool{false, false, false, false},
		},
	}
}

type IntegrationTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network
}

func NewIntegrationTestSuite(cfg network.Config) *IntegrationTestSuite {
	return &IntegrationTestSuite{cfg: cfg}
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	genState := app.ModuleBasics.DefaultGenesis(s.cfg.Codec)
	claimGenState := claimtypes.DefaultGenesis()
	claimGenState.ModuleAccountBalance = sdk.NewCoin(claimtypes.DefaultClaimDenom, sdk.NewInt(30))
	claimGenState.ClaimRecords = claimRecords
	claimGenStateBz := s.cfg.Codec.MustMarshalJSON(claimGenState)
	genState[claimtypes.ModuleName] = claimGenStateBz

	s.cfg.GenesisState = genState
	s.network = network.New(s.T(), s.cfg)

	_, err := s.network.WaitForHeight(1)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestCmdQueryClaimRecord() {
	val := s.network.Validators[0]

	testCases := []struct {
		name string
		args []string
	}{
		{
			"query claim record",
			[]string{
				addr1.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryClaimRecord()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)

			var result claimtypes.QueryClaimRecordResponse
			s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &result))
			s.Require().Equal(result.GetClaimRecord(), claimRecords[0])
		})
	}
}

func (s *IntegrationTestSuite) TestCmdQueryClaimableForAction() {
	val := s.network.Validators[0]

	testCases := []struct {
		name  string
		args  []string
		coins sdk.Coins
	}{
		{
			"query claimable-for-action amount",
			[]string{
				addr2.String(),
				claimtypes.Action_name[0],
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			sdk.Coins{sdk.NewCoin(claimtypes.DefaultClaimDenom, sdk.NewInt(20))},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryClaimableForAction()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)

			var result claimtypes.QueryClaimableForActionResponse
			s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &result))
			s.Require().Equal(result.Amount.String(), tc.coins.String())
		})
	}
}

func (s *IntegrationTestSuite) TestCmdQueryModuleAccountBalance() {
	val := s.network.Validators[0]

	testCases := []struct {
		name  string
		args  []string
		coins sdk.Coins
	}{
		{
			"query module-account-balance",
			[]string{
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			sdk.Coins{sdk.NewCoin(claimtypes.DefaultClaimDenom, sdk.NewInt(30))},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryModuleAccountBalance()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)

			var result claimtypes.QueryModuleAccountBalanceResponse
			s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &result))
			s.Require().Equal(result.ModuleAccountBalance.String(), tc.coins.String())
		})
	}
}
