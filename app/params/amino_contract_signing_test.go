package params_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	txsigning "cosmossdk.io/x/tx/signing"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/envadiv/Passage3D/app"
	"github.com/stretchr/testify/require"
)

// Amino clients (Ledger, Keplr/Leap Amino signers, CosmJS AminoTypes) build the
// StdSignDoc themselves, with RawContractMessage fields inlined as JSON and every
// object's keys sorted, then sign those bytes. The node must rebuild exactly the
// same bytes or SigVerificationDecorator rejects the tx with code 4
// ("signature verification failed"). wasmd v0.50.0 rebuilt `msg` as base64 and empty
// `funds` as null; fixed in wasmd v0.51 (CosmWasm/wasmd#1844, #1860), shipped by v0.54.10.
func TestAminoSignBytesForWasmMessagesMatchAminoClients(t *testing.T) {
	encCfg := app.MakeEncodingConfig()
	txCfg := encCfg.TxConfig

	priv := secp256k1.GenPrivKey()
	sender := sdk.AccAddress(priv.PubKey().Address())
	contract := sdk.AccAddress([]byte("marketplace_contract_addr_32byte"))

	cases := []struct {
		name     string
		msg      sdk.Msg
		expected string
	}{
		{
			name: "MsgExecuteContract with funds (set_bid)",
			msg: &wasmtypes.MsgExecuteContract{
				Sender:   sender.String(),
				Contract: contract.String(),
				// Unsorted on purpose: clients sort the inner object too.
				Msg:   []byte(`{"set_bid":{"token_id":"1","price":{"denom":"upasg","amount":"5000000"},"expires_at":"0"}}`),
				Funds: sdk.NewCoins(sdk.NewInt64Coin("upasg", 5000000)),
			},
			expected: `{"account_number":"7","chain_id":"passage-2","fee":{"amount":[{"amount":"5000000","denom":"upasg"}],"gas":"400000"},"memo":"","msgs":[{"type":"wasm/MsgExecuteContract","value":{"contract":"` + contract.String() + `","funds":[{"amount":"5000000","denom":"upasg"}],"msg":{"set_bid":{"expires_at":"0","price":{"amount":"5000000","denom":"upasg"},"token_id":"1"}},"sender":"` + sender.String() + `"}}],"sequence":"3"}`,
		},
		{
			name: "MsgExecuteContract without funds (remove_ask)",
			msg: &wasmtypes.MsgExecuteContract{
				Sender:   sender.String(),
				Contract: contract.String(),
				Msg:      []byte(`{"remove_ask":{"token_id":"1"}}`),
			},
			expected: `{"account_number":"7","chain_id":"passage-2","fee":{"amount":[{"amount":"5000000","denom":"upasg"}],"gas":"400000"},"memo":"","msgs":[{"type":"wasm/MsgExecuteContract","value":{"contract":"` + contract.String() + `","funds":[],"msg":{"remove_ask":{"token_id":"1"}},"sender":"` + sender.String() + `"}}],"sequence":"3"}`,
		},
		{
			name: "MsgInstantiateContract",
			msg: &wasmtypes.MsgInstantiateContract{
				Sender: sender.String(),
				CodeID: 12,
				Label:  "pg721",
				Msg:    []byte(`{"name":"Collection","symbol":"COL"}`),
			},
			expected: `{"account_number":"7","chain_id":"passage-2","fee":{"amount":[{"amount":"5000000","denom":"upasg"}],"gas":"400000"},"memo":"","msgs":[{"type":"wasm/MsgInstantiateContract","value":{"code_id":"12","funds":[],"label":"pg721","msg":{"name":"Collection","symbol":"COL"},"sender":"` + sender.String() + `"}}],"sequence":"3"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			builder := txCfg.NewTxBuilder()
			require.NoError(t, builder.SetMsgs(tc.msg))
			builder.SetGasLimit(400000)
			builder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin("upasg", math.NewInt(5000000))))

			// Placeholder signature so the signer info (pubkey, mode, sequence) is in the tx.
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
			require.Equal(t, tc.expected, string(signBytes))

			// Sign the client-built bytes and verify with the node's handler,
			// the same check SigVerificationDecorator performs.
			sig, err := priv.Sign([]byte(tc.expected))
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
		})
	}
}
