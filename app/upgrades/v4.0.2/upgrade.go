package v4_0_2

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	"github.com/cosmos/cosmos-sdk/x/group"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
)

// Name is the on-chain upgrade name for Passage v4.0.2.
//
// IMPORTANT: this string MUST exactly equal the plan.name in the gov proposal
// (a plan-name mismatch is what got Passage prop #21 rejected). The fork-test
// asserts the node logs `applying upgrade "v4.0.2"` before any release is tagged.
const Name = "v4.0.2"

// Upgrade: adds the x/group store. group.AppModuleBasic.DefaultGenesis /
// group.AppModule.InitGenesis run automatically inside RunMigrations below,
// because "group" is absent from fromVM (RunMigrations treats any module
// missing from the on-disk version map as newly added and calls its
// InitGenesis with DefaultGenesis instead of a migration).
var Upgrade = upgrades.Upgrade{
	UpgradeName:          Name,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: storetypes.StoreUpgrades{
		Added: []string{group.ModuleName},
	},
}

// CreateUpgradeHandler matches the shared upgrades.Upgrade signature. All
// keepers other than mm/configurator are unused: v4.0.2 makes no param or
// state changes beyond the new x/group store, which RunMigrations handles.
func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	_ distribution.Keeper,
	_ bank.Keeper,
	_ auth.AccountKeeper,
	_ claim.Keeper,
	_ consensusparamkeeper.Keeper,
	_ paramskeeper.Keeper,
	_ *stakingkeeper.Keeper,
	_ govkeeper.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
