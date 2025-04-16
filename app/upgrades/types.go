package upgrades

import (
	"github.com/cosmos/cosmos-sdk/codec"
	store "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authz "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	feegrant "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	gov "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
)

// Upgrade defines a struct containing necessary fields that a SoftwareUpgradeProposal
// must have written, in order for the state migration to go smoothly.
// An upgrade must implement this struct, and then set it in the app.go.
// The app.go will then define the handler.
type Upgrade struct {
	// Upgrade version name, for the upgrade handler, e.g. `v1`
	UpgradeName string

	// CreateUpgradeHandler defines the function that creates an upgrade handler
	CreateUpgradeHandler func(*module.Manager, module.Configurator, codec.Codec, distribution.Keeper,
		bank.Keeper, auth.AccountKeeper, staking.Keeper, gov.Keeper, authz.Keeper, feegrant.Keeper, claim.Keeper,
	) upgradetypes.UpgradeHandler

	// Store upgrades, should be used for any new modules introduced, new modules deleted, or store names renamed.
	StoreUpgrades store.StoreUpgrades
}
