package v047

import (
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authz "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	feegrant "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	gov "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
)

// Name is the on-chain upgrade name for the Cosmos SDK v0.45 -> v0.47 migration.
const Name = "v047"

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
	_ codec.Codec,
	_ distribution.Keeper,
	_ bank.Keeper,
	_ auth.AccountKeeper,
	_ staking.Keeper,
	_ gov.Keeper,
	_ authz.Keeper,
	_ feegrant.Keeper,
	_ claim.Keeper,
	consensusParamsKeeper consensusparamkeeper.Keeper,
	paramsKeeper paramskeeper.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("v047 upgrade: migrating Tendermint consensus params x/params -> x/consensus")

		// The "baseapp" subspace is not registered by initParamsKeeper, so a fresh
		// Subspace call is safe here (it would panic if already occupied).
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
