package ante_test

import (
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/envadiv/Passage3D/ante"
	claimtypes "github.com/envadiv/Passage3D/x/claim/types"
)

func (s *IntegrationTestSuite) TestMempoolFeeDecorator() {
	s.SetupTest()
	s.txBuilder = s.clientCtx.TxConfig.NewTxBuilder()

	mfd := ante.NewMempoolFeeDecorator()
	antehandler := sdk.ChainAnteDecorators(mfd)
	priv1, _, addr1 := testdata.KeyTestPubAddr()

	msg := banktypes.NewMsgSend(addr1, addr1, sdk.NewCoins(sdk.NewCoin("upasg", sdk.NewInt(12123123213))))
	feeAmount := sdk.NewCoins(sdk.NewInt64Coin("pasg", 150))
	gasLimit := uint64(200000)
	s.Require().NoError(s.txBuilder.SetMsgs(msg))
	s.txBuilder.SetFeeAmount(feeAmount)
	s.txBuilder.SetGasLimit(gasLimit)

	privs, accNums, accSeqs := []cryptotypes.PrivKey{priv1}, []uint64{0}, []uint64{0}
	tx, err := s.CreateTestTx(privs, accNums, accSeqs, s.ctx.ChainID())
	s.Require().NoError(err)

	// Set high gas price so standard test fee fails
	feeAmt := sdk.NewDecCoinFromDec("upasg", sdk.NewDec(200).Quo(sdk.NewDec(100000)))
	minGasPrice := []sdk.DecCoin{feeAmt}
	s.ctx = s.ctx.WithMinGasPrices(minGasPrice).WithIsCheckTx(true)

	// antehandler errors with insufficient fees
	_, err = antehandler(s.ctx, tx, false)
	s.Require().Error(err, "expected error due to low fee")

	// ensure no fees for certain Claim msgs
	s.Require().NoError(s.txBuilder.SetMsgs(
		claimtypes.NewMsgClaim(addr1.String(), "ActionInitialClaim"),
	))

	oracleTx, err := s.CreateTestTx(privs, accNums, accSeqs, s.ctx.ChainID())
	_, err = antehandler(s.ctx, oracleTx, false)
	s.Require().NoError(err, "expected min fee bypass for IBC messages")

	s.ctx = s.ctx.WithIsCheckTx(false)

	// antehandler should not error since we do not check min gas prices in DeliverTx
	_, err = antehandler(s.ctx, tx, false)
	s.Require().NoError(err, "unexpected error during DeliverTx")

}
