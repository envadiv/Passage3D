package app

import (
	"github.com/envadiv/Passage3D/app/params"

	"github.com/cosmos/cosmos-sdk/std"
)

// MakeEncodingConfig creates an EncodingConfig for testing.
func MakeEncodingConfig() params.EncodingConfig {
	encodingConfig := params.MakeTestEncodingConfig()
	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ModuleBasics.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	return encodingConfig
}
