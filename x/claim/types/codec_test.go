package types_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/x/claim/types"
)

// Since SDK v0.50 signers are resolved from the cosmos.msg.v1.signer proto
// option. Without it every MsgClaim tx failed with
// "no cosmos.msg.v1.signer option found".
func TestMsgClaimSigners(t *testing.T) {
	encCfg := app.MakeEncodingConfig()
	sender := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	msg := types.NewMsgClaim(sender.String(), types.InitialClaim)

	signers, _, err := encCfg.Marshaler.GetMsgV1Signers(msg)
	require.NoError(t, err)
	require.Equal(t, [][]byte{sender}, signers)

	builder := encCfg.TxConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	txSigners, err := builder.GetTx().GetSigners()
	require.NoError(t, err)
	require.Equal(t, [][]byte{sender}, txSigners)
}

// Amino JSON clients (Ledger, Keplr/Leap Amino signers) sign MsgClaim with the
// "claim/Claim" type name used since v2.x; the node must rebuild the same bytes.
func TestMsgClaimAminoJSONSignBytes(t *testing.T) {
	txCfg := app.MakeEncodingConfig().TxConfig
	priv := secp256k1.GenPrivKey()
	sender := sdk.AccAddress(priv.PubKey().Address())

	builder := txCfg.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(types.NewMsgClaim(sender.String(), types.InitialClaim)))
	builder.SetGasLimit(200000)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("upasg", 2500000)))

	sigData := &signingtypes.SingleSignatureData{SignMode: signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON}
	require.NoError(t, builder.SetSignatures(signingtypes.SignatureV2{PubKey: priv.PubKey(), Data: sigData, Sequence: 3}))

	signerData := authsigning.SignerData{
		Address:       sender.String(),
		ChainID:       "passage-2",
		AccountNumber: 7,
		Sequence:      3,
		PubKey:        priv.PubKey(),
	}
	signBytes, err := authsigning.GetSignBytesAdapter(
		context.Background(), txCfg.SignModeHandler(),
		signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, signerData, builder.GetTx(),
	)
	require.NoError(t, err)

	expected := `{"account_number":"7","chain_id":"passage-2","fee":{"amount":[{"amount":"2500000","denom":"upasg"}],"gas":"200000"},"memo":"","msgs":[{"type":"claim/Claim","value":{"claim_action":"ActionInitialClaim","sender":"` + sender.String() + `"}}],"sequence":"3"}`
	require.Equal(t, expected, string(signBytes))

	// A signature over the client-built bytes verifies with the node's handler.
	sig, err := priv.Sign([]byte(expected))
	require.NoError(t, err)
	sigData.Signature = sig
	require.NoError(t, builder.SetSignatures(signingtypes.SignatureV2{PubKey: priv.PubKey(), Data: sigData, Sequence: 3}))

	adaptable, ok := builder.GetTx().(authsigning.V2AdaptableTx)
	require.True(t, ok)
	require.NoError(t, authsigning.VerifySignature(
		context.Background(), priv.PubKey(),
		txsigning.SignerData{Address: sender.String(), ChainID: "passage-2", AccountNumber: 7, Sequence: 3},
		sigData, txCfg.SignModeHandler(), adaptable.GetSigningTxData(),
	))
}
