package v4_0_1

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
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
)

// Name is the on-chain upgrade name for the Passage v4.0.1 emergency fix.
// v4.0.1 fixes three registry defects shipped in v4.0.0 — all at the binary
// level: the InterfaceRegistry address codec, ibctm/solomachine light-client
// registration in ModuleBasics, and the cosmwasm_1_3 wasm capability.
//
// IMPORTANT: this string MUST exactly equal the plan.name in the gov proposal
// (a mismatch is what got Passage prop #21 rejected). The fork-test asserts the
// node logs `applying upgrade "v4.0.1"` before any release is tagged.
const Name = "v4.0.1"

// Upgrade: no new/renamed/deleted module stores (the fixes are code-only), so
// StoreUpgrades is empty. There is therefore no store-divergence risk at the height.
var Upgrade = upgrades.Upgrade{
	UpgradeName:          Name,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{},
}

// CreateUpgradeHandler matches the shared upgrades.Upgrade signature. All keepers
// are unused: v4.0.1 makes no state or param changes (the staking min-commission
// and gov expedited params were already set by the v4.0.0 handler).
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
		// No module consensus-version bumps: RunMigrations is a no-op that simply
		// records the current module versions. It is still the correct call — it
		// makes the handler future-proof if a module version ever changes and keeps
		// the version map consistent.
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
