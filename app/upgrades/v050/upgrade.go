package v050

import (
	"context"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	circuittypes "cosmossdk.io/x/circuit/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

// Name is the on-chain upgrade name for the SDK v0.47 -> v0.50 migration.
const Name = "v050"

// Upgrade migrates the chain from Cosmos SDK v0.47 to v0.50. StoreUpgrades.Added
// is set in app.go alongside any new modules (e.g. x/circuit).
var Upgrade = upgrades.Upgrade{
	UpgradeName:          Name,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{Added: []string{circuittypes.StoreKey}},
}

func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	_ distribution.Keeper,
	_ bank.Keeper,
	_ auth.AccountKeeper,
	_ claim.Keeper,
	_ consensusparamkeeper.Keeper,
	_ paramskeeper.Keeper,
	stakingKeeper *stakingkeeper.Keeper,
	govKeeper govkeeper.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		vm, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return vm, err
		}
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		minRate := math.LegacyNewDecWithPrec(5, 2) // 5%

		// x/staking: set minimum validator commission to 5% and lift existing
		// validators that sit below the new floor.
		sparams, err := stakingKeeper.GetParams(ctx)
		if err != nil {
			return vm, err
		}
		sparams.MinCommissionRate = minRate
		if err := stakingKeeper.SetParams(ctx, sparams); err != nil {
			return vm, err
		}
		vals, err := stakingKeeper.GetAllValidators(ctx)
		if err != nil {
			return vm, err
		}
		for _, v := range vals {
			changed := false
			if v.Commission.MaxRate.LT(minRate) {
				v.Commission.MaxRate = minRate
				changed = true
			}
			if v.Commission.Rate.LT(minRate) {
				v.Commission.Rate = minRate
				v.Commission.UpdateTime = sdkCtx.BlockTime()
				changed = true
			}
			if changed {
				if err := stakingKeeper.SetValidator(ctx, v); err != nil {
					return vm, err
				}
			}
		}

		// x/gov: expedited + shared governance params.
		// NOTE: SDK 0.50 gov has NO separate expedited quorum/veto — Quorum and
		// VetoThreshold are shared by both normal and expedited proposals.
		gparams, err := govKeeper.Params.Get(ctx)
		if err != nil {
			return vm, err
		}
		expeditedVoting := 24 * time.Hour
		gparams.ExpeditedVotingPeriod = &expeditedVoting
		gparams.ExpeditedThreshold = math.LegacyNewDecWithPrec(67, 2).String()                                     // 67%
		gparams.Quorum = math.LegacyNewDecWithPrec(50, 2).String()                                                 // 50% (shared)
		gparams.VetoThreshold = math.LegacyNewDecWithPrec(50, 2).String()                                          // 50% (shared)
		gparams.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewCoin(sparams.BondDenom, math.NewInt(1_000_000_000_000))) // 1,000,000 PASG (bond denom = upasg on mainnet)
		if err := govKeeper.Params.Set(ctx, gparams); err != nil {
			return vm, err
		}

		return vm, nil
	}
}
