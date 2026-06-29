package v3_0_0

import (
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"

	"context"
)

// Name is the on-chain upgrade name for the Cosmos SDK v0.45 -> v0.47 migration.
const Name = "v3.0.0"

// Upgrade migrates passage-2 from cosmos-sdk v0.45 (wasmd 0.34 / wasmvm 1.5.7) to
// cosmos-sdk v0.47 (wasmd 0.45 / wasmvm 1.5.x). wasmvm stays in the 1.5 line so
// contract state is untouched. Two new module stores are created at the height:
// x/consensus (Tendermint consensus params) and x/crisis (constant fee).
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

// CreateUpgradeHandler returns the v0.47 upgrade handler. Only the consensus-params
// and params keepers are used (to migrate Tendermint consensus params out of the
// legacy x/params subspace); the rest are part of the shared handler signature.
func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	_ distribution.Keeper,
	_ bank.Keeper,
	_ auth.AccountKeeper,
	_ claim.Keeper,
	consensusParamsKeeper consensusparamkeeper.Keeper,
	paramsKeeper paramskeeper.Keeper,
	_ *stakingkeeper.Keeper,
	_ govkeeper.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.Logger().Info("v3.0.0 upgrade: migrating Tendermint consensus params x/params -> x/consensus")

		// The "baseapp" subspace is not registered by initParamsKeeper, so a fresh
		// Subspace call is safe here (it would panic if already occupied).
		legacyBaseAppSubspace := paramsKeeper.
			Subspace(baseapp.Paramspace).
			WithKeyTable(paramstypes.ConsensusParamsKeyTable())
		if err := baseapp.MigrateParams(sdkCtx, legacyBaseAppSubspace, &consensusParamsKeeper); err != nil {
			return nil, err
		}

		sdkCtx.Logger().Info("v3.0.0 upgrade: running module migrations")
		vm, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		sdkCtx.Logger().Info("v3.0.0 upgrade: complete")
		return vm, nil
	}
}
