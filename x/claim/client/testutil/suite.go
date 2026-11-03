package testutil

import (
	"fmt"

	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/envadiv/Passage3D/testutil/network"
	"github.com/envadiv/Passage3D/x/claim/client/cli"
	claimtypes "github.com/envadiv/Passage3D/x/claim/types"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network

	// addr1 and addr2 have claim records but no keys; claimer is the first
	// validator's operator account, which can sign claim transactions.
	addr1, addr2, claimer sdk.AccAddress
	claimRecords          []claimtypes.ClaimRecord
}

func NewIntegrationTestSuite(cfg network.Config) *IntegrationTestSuite {
	return &IntegrationTestSuite{cfg: cfg}
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	// Fix the first validator's mnemonic so its address can get a claim record.
	kr := keyring.NewInMemory(s.cfg.Codec)
	record, mnemonic, err := kr.NewMnemonic("claimer", keyring.English, sdk.GetConfig().GetFullBIP44Path(),
		keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)
	s.claimer, err = record.GetAddress()
	s.Require().NoError(err)
	s.cfg.Mnemonics = []string{mnemonic}

	s.addr1 = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	s.addr2 = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	s.claimRecords = []claimtypes.ClaimRecord{
		{
			Address:         s.addr1.String(),
			ClaimableAmount: sdk.NewCoins(sdk.NewInt64Coin(claimtypes.DefaultClaimDenom, 10)),
			ActionCompleted: []bool{false},
		},
		{
			Address:         s.addr2.String(),
			ClaimableAmount: sdk.NewCoins(sdk.NewInt64Coin(claimtypes.DefaultClaimDenom, 20)),
			ActionCompleted: []bool{false},
		},
		{
			Address:         s.claimer.String(),
			ClaimableAmount: sdk.NewCoins(sdk.NewInt64Coin(claimtypes.DefaultClaimDenom, 30)),
			ActionCompleted: []bool{false},
		},
	}

	claimGenState := claimtypes.DefaultGenesis()
	claimGenState.Params.AirdropEnabled = true
	claimGenState.ModuleAccountBalance = sdk.NewCoin(claimtypes.DefaultClaimDenom, sdkmath.NewInt(60))
	claimGenState.ClaimRecords = s.claimRecords
	s.Require().NoError(claimGenState.Validate())
	s.cfg.GenesisState[claimtypes.ModuleName] = s.cfg.Codec.MustMarshalJSON(claimGenState)

	s.network = network.New(s.T(), s.cfg)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestCmdQueryParams() {
	clientCtx := s.network.Validators[0].ClientCtx

	out, err := clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryParams(), []string{fmt.Sprintf("--%s=json", flags.FlagOutput)})
	s.Require().NoError(err)

	// The command prints the params themselves, not the query response.
	var params claimtypes.Params
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &params))
	s.Require().True(params.AirdropEnabled)
	s.Require().Equal(claimtypes.DefaultClaimDenom, params.ClaimDenom)
	s.Require().False(params.AirdropStartTime.IsZero(), "InitGenesis sets the start time to the genesis time")
}

func (s *IntegrationTestSuite) TestCmdQueryClaimRecord() {
	clientCtx := s.network.Validators[0].ClientCtx

	out, err := clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryClaimRecord(), []string{
		s.addr1.String(),
		fmt.Sprintf("--%s=json", flags.FlagOutput),
	})
	s.Require().NoError(err)

	var result claimtypes.QueryClaimRecordResponse
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &result))
	s.Require().Equal(s.claimRecords[0], result.GetClaimRecord())
}

func (s *IntegrationTestSuite) TestCmdQueryClaimableForAction() {
	clientCtx := s.network.Validators[0].ClientCtx

	out, err := clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryClaimableForAction(), []string{
		s.addr2.String(),
		claimtypes.Action_name[claimtypes.ActionInitialClaim],
		fmt.Sprintf("--%s=json", flags.FlagOutput),
	})
	s.Require().NoError(err)

	var result claimtypes.QueryClaimableForActionResponse
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &result))
	s.Require().Equal(sdk.NewInt64Coin(claimtypes.DefaultClaimDenom, 20).String(), result.Amount.String())
}

func (s *IntegrationTestSuite) TestCmdQueryTotalClaimable() {
	clientCtx := s.network.Validators[0].ClientCtx

	out, err := clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryTotalClaimable(), []string{
		s.addr1.String(),
		fmt.Sprintf("--%s=json", flags.FlagOutput),
	})
	s.Require().NoError(err)

	var result claimtypes.QueryTotalClaimableResponse
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &result))
	s.Require().Equal(sdk.NewCoins(s.claimRecords[0].ClaimableAmount...).String(), result.Coins.String())
}

func (s *IntegrationTestSuite) TestCmdQueryModuleAccountBalance() {
	clientCtx := s.network.Validators[0].ClientCtx

	out, err := clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryModuleAccountBalance(), []string{
		fmt.Sprintf("--%s=json", flags.FlagOutput),
	})
	s.Require().NoError(err)

	var result claimtypes.QueryModuleAccountBalanceResponse
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &result))
	// The claim transaction test may already have run.
	s.Require().Contains([]string{"60upasg", "30upasg"}, result.ModuleAccountBalance.String())
}

func (s *IntegrationTestSuite) TestCmdTxClaim() {
	val := s.network.Validators[0]
	s.Require().Equal(s.claimer, val.Address)
	clientCtx := val.ClientCtx

	// The minimum gas price (12.5upasg) applies to the default gas limit.
	fees := sdk.NewCoins(sdk.NewInt64Coin(s.cfg.BondDenom, flags.DefaultGasLimit*125/10))
	txArgs := []string{
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address),
		fmt.Sprintf("--%s=%s", flags.FlagFees, fees),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=json", flags.FlagOutput),
	}

	testCases := []struct {
		name      string
		action    string
		expectErr string
	}{
		{"unknown action", "ActionUnknown", "invalid action type"},
		{"initial claim", claimtypes.InitialClaim, ""},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			out, err := clitestutil.ExecTestCLICmd(clientCtx, cli.CmdInitialClaim(), append([]string{tc.action}, txArgs...))
			if tc.expectErr != "" {
				s.Require().ErrorContains(err, tc.expectErr)
				return
			}
			s.Require().NoError(err)

			var txResp sdk.TxResponse
			s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &txResp), out.String())
			s.Require().Equal(uint32(0), txResp.Code, txResp.RawLog)
			s.Require().NoError(clitestutil.CheckTxCode(s.network, clientCtx, txResp.TxHash, 0))
		})
	}

	out, err := clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryClaimRecord(), []string{
		val.Address.String(),
		fmt.Sprintf("--%s=json", flags.FlagOutput),
	})
	s.Require().NoError(err)
	var result claimtypes.QueryClaimRecordResponse
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &result))
	s.Require().Equal([]bool{true}, result.ClaimRecord.ActionCompleted)

	out, err = clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryModuleAccountBalance(), []string{
		fmt.Sprintf("--%s=json", flags.FlagOutput),
	})
	s.Require().NoError(err)
	var balance claimtypes.QueryModuleAccountBalanceResponse
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &balance))
	s.Require().Equal("30upasg", balance.ModuleAccountBalance.String())
}
