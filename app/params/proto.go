//go:build !test_amino
// +build !test_amino

package params

import (
	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"
)

// MakeTestEncodingConfig creates an EncodingConfig for a non-amino based configuration.
//
// NOTE: despite the "Test" name this is the encoding config the app and CLI actually
// use (app.MakeEncodingConfig calls it). Under Cosmos SDK v0.50 the InterfaceRegistry
// MUST be constructed with an address codec via SigningOptions; otherwise any message
// carrying a validator (…valoper) address — MsgDelegate, MsgUndelegate,
// MsgBeginRedelegate, MsgWithdrawDelegatorReward — fails to build or process with:
//
//	"InterfaceRegistry requires a proper address codec implementation to do address conversion"
//
// The plain types.NewInterfaceRegistry() used previously has no codec, which silently
// breaks all staking/distribution transactions network-wide (bank sends still work).
//
// Prefixes are the literals from app/addr_prefixes.go, used directly here rather than
// sdk.GetConfig() to avoid coupling this package's init order to the app package.
func MakeTestEncodingConfig() EncodingConfig {
	cdc := codec.NewLegacyAmino()
	interfaceRegistry, err := types.NewInterfaceRegistryWithOptions(types.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          authcodec.NewBech32Codec("pasg"),
			ValidatorAddressCodec: authcodec.NewBech32Codec("pasgvaloper"),
		},
	})
	if err != nil {
		panic(err)
	}
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          tx.NewTxConfig(marshaler, tx.DefaultSignModes),
		Amino:             cdc,
	}
}
