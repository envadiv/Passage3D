package claim_test

import (
	"math/rand"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"

	"github.com/envadiv/Passage3D/app"
	passageante "github.com/envadiv/Passage3D/app/ante"
	"github.com/envadiv/Passage3D/x/claim"
	"github.com/envadiv/Passage3D/x/claim/types"
)

// TestMsgClaimTxThroughApp sends a signed MsgClaim through CheckTx and
// FinalizeBlock. Before MsgClaim declared cosmos.msg.v1.signer, the tx was
// rejected while resolving its signers.
func TestMsgClaimTxThroughApp(t *testing.T) {
	a := app.Setup(false, t)
	// Passage nodes run with at least this minimum gas price, which the ante
	// handler enforces in CheckTx. It is applied to the CheckTx state at Commit.
	baseapp.SetMinGasPrices(passageante.DefaultMinGasPrice + passageante.BaseDenom)(a.BaseApp)

	const gas = 200000
	fees := sdk.NewCoins(sdk.NewInt64Coin(app.BondDenom, gas*125/10))
	claimable := sdk.NewInt64Coin(types.DefaultClaimDenom, 1000)

	priv := secp256k1.GenPrivKey()
	claimer := sdk.AccAddress(priv.PubKey().Address())

	ctx := deliverCtx(a, now)
	require.NoError(t, banktestutil.FundAccount(ctx, a.BankKeeper, claimer, fees))
	genesis := testGenesis()
	genesis.ClaimRecords = append(genesis.ClaimRecords, types.ClaimRecord{
		Address:         claimer.String(),
		ClaimableAmount: sdk.NewCoins(claimable),
		ActionCompleted: []bool{false},
	})
	genesis.ModuleAccountBalance = genesis.ModuleAccountBalance.Add(claimable)
	claim.InitGenesis(ctx, a.ClaimKeeper, genesis)
	finalizeAndCommit(t, a, now.Add(time.Second))

	checkCtx := a.BaseApp.NewContext(true)
	acc := a.AccountKeeper.GetAccount(checkCtx, claimer)
	require.NotNil(t, acc)

	txCfg := app.MakeEncodingConfig().TxConfig
	tx, err := simtestutil.GenSignedMockTx(
		rand.New(rand.NewSource(1)), txCfg,
		[]sdk.Msg{types.NewMsgClaim(claimer.String(), types.InitialClaim)},
		fees, gas, a.ChainID(),
		[]uint64{acc.GetAccountNumber()}, []uint64{acc.GetSequence()},
		priv,
	)
	require.NoError(t, err)
	txBytes, err := txCfg.TxEncoder()(tx)
	require.NoError(t, err)

	checkRes, err := a.CheckTx(&abci.RequestCheckTx{Tx: txBytes, Type: abci.CheckTxType_New})
	require.NoError(t, err)
	require.Equal(t, uint32(0), checkRes.Code, checkRes.Log)

	res := finalizeAndCommit(t, a, now.Add(2*time.Second), txBytes)
	require.Len(t, res.TxResults, 1)
	require.Equal(t, uint32(0), res.TxResults[0].Code, res.TxResults[0].Log)

	checkCtx = a.BaseApp.NewContext(true)
	// The fee was spent and the claimable amount received.
	require.Equal(t, sdk.NewCoins(claimable).String(), a.BankKeeper.GetAllBalances(checkCtx, claimer).String())
	record, err := a.ClaimKeeper.GetClaimRecord(checkCtx, claimer)
	require.NoError(t, err)
	require.Equal(t, []bool{true}, record.ActionCompleted)
}
