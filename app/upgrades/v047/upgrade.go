package v047

import (
	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
)

// Name is the on-chain upgrade name for the Cosmos SDK v0.45 -> v0.47 migration.
const Name = "v047"

// Upgrade migrates the chain from Cosmos SDK v0.45 to v0.47.
//
// Two new module stores are introduced in v0.47 and must be created at the
// upgrade height:
//   - x/consensus: the dedicated home for Tendermint consensus params, which
//     previously lived in the x/params "baseapp" subspace.
//   - x/crisis: the constant fee moved from the x/params subspace into the
//     module's own store.
var Upgrade = upgrades.Upgrade{
	UpgradeName:          Name,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: storetypes.StoreUpgrades{
		Added: []string{
			consensusparamtypes.StoreKey,
			crisistypes.StoreKey,
		},
	},
}

// CreateUpgradeHandler returns the v0.47 upgrade handler. The distribution,
// bank, auth and claim keepers are unused for this purely-technical SDK bump;
// the consensus-params and params keepers are required to migrate the
// Tendermint consensus params out of the legacy x/params subspace.
func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	_ distribution.Keeper,
	_ bank.Keeper,
	_ auth.AccountKeeper,
	_ claim.Keeper,
	consensusParamsKeeper consensusparamkeeper.Keeper,
	paramsKeeper paramskeeper.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("v047 upgrade: migrating Tendermint consensus params x/params -> x/consensus")

		// The "baseapp" subspace is not registered by initParamsKeeper, so a
		// fresh Subspace call is safe here (it would panic if already occupied).
		legacyBaseAppSubspace := paramsKeeper.
			Subspace(baseapp.Paramspace).
			WithKeyTable(paramstypes.ConsensusParamsKeyTable())
		baseapp.MigrateParams(ctx, legacyBaseAppSubspace, &consensusParamsKeeper)

		ctx.Logger().Info("v047 upgrade: running module migrations")
		vm, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		ctx.Logger().Info("v047 upgrade: complete")
		return vm, nil
	}
}
