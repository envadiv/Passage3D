package ante

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	codectestutil "github.com/cosmos/cosmos-sdk/codec/testutil"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/stretchr/testify/require"
)

func init() {
	// handleMessages compares bech32 strings built with the global prefix.
	sdk.GetConfig().SetBech32PrefixForAccount("pasg", "pasgpub")
}

func newTestCodec(t *testing.T) codec.Codec {
	t.Helper()
	ir := codectestutil.CodecOptions{AccAddressPrefix: "pasg", ValAddressPrefix: "pasgvaloper"}.NewInterfaceRegistry()
	std.RegisterInterfaces(ir)
	banktypes.RegisterInterfaces(ir)
	distrtypes.RegisterInterfaces(ir)
	authz.RegisterInterfaces(ir)
	return codec.NewProtoCodec(ir)
}

func TestBlockAccountDecorator(t *testing.T) {
	cdc := newTestCodec(t)
	other := sdk.AccAddress([]byte("some_other_account__")).String()
	coins := sdk.NewCoins(sdk.NewInt64Coin("upasg", 1))

	sendFromBlocked := banktypes.NewMsgSend(sdk.MustAccAddressFromBech32(blockedMultisigAddr), sdk.MustAccAddressFromBech32(other), coins)
	fundFromBlocked := &distrtypes.MsgFundCommunityPool{Depositor: blockedMultisigAddr, Amount: coins}
	sendFromOther := banktypes.NewMsgSend(sdk.MustAccAddressFromBech32(other), sdk.MustAccAddressFromBech32(blockedMultisigAddr), coins)
	execSendFromBlocked := authz.NewMsgExec(sdk.MustAccAddressFromBech32(other), []sdk.Msg{sendFromBlocked})

	cases := []struct {
		name    string
		msgs    []sdk.Msg
		blocked bool
	}{
		{"send from blocked account", []sdk.Msg{sendFromBlocked}, true},
		{"fund community pool from blocked account", []sdk.Msg{fundFromBlocked}, false},
		// Before v4.0.4 the MsgFundCommunityPool returned early and the
		// following message was never checked.
		{"fund community pool followed by send from blocked account", []sdk.Msg{fundFromBlocked, sendFromBlocked}, true},
		{"authz exec of a send from blocked account", []sdk.Msg{&execSendFromBlocked}, true},
		{"send from another account", []sdk.Msg{sendFromOther}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := handleMessages(cdc, tc.msgs)
			if tc.blocked {
				require.Error(t, err)
				require.Contains(t, err.Error(), "is not allowed to perform this transaction")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMinGasPricesDecorator(t *testing.T) {
	dec := NewValidateMinGasPricesDecorator()
	next := func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return ctx, nil }
	price := func(p string) sdk.DecCoins {
		return sdk.NewDecCoins(sdk.NewDecCoinFromDec(BaseDenom, math.LegacyMustNewDecFromStr(p)))
	}

	cases := []struct {
		name     string
		checkTx  bool
		simulate bool
		minGas   sdk.DecCoins
		wantErr  bool
	}{
		{"CheckTx below floor", true, false, price("1"), true},
		{"CheckTx without min gas price", true, false, sdk.DecCoins{}, true},
		{"CheckTx at floor", true, false, price(DefaultMinGasPrice), false},
		{"CheckTx above floor", true, false, price("20"), false},
		{"simulation below floor", true, true, price("1"), false},
		{"DeliverTx below floor (not a consensus rule)", false, false, price("1"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := sdk.Context{}.WithIsCheckTx(tc.checkTx).WithMinGasPrices(tc.minGas)
			_, err := dec.AnteHandle(ctx, nil, tc.simulate, next)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
