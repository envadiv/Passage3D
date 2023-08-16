package v1

import (
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/envadiv/Passage3D/app/upgrades"
)

const Name = "v1.0"
const upasgDenom = "upasg"

var amount = sdk.NewCoins(sdk.NewCoin(upasgDenom, sdk.NewInt(150000000000000)))

var Upgrade = upgrades.Upgrade{
	UpgradeName:          Name,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{},
}

func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	dk distribution.Keeper,
	bk bank.Keeper,
	ak auth.AccountKeeper,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		if err := ExecuteProposal(ctx, dk, bk, ak); err != nil {
			return nil, err
		}

		return fromVM, nil
	}
}

// ExecuteProposal moves community pool funds to a multisig vesting account
func ExecuteProposal(ctx sdk.Context, dk distribution.Keeper, bk bank.Keeper, ak auth.AccountKeeper) error {
	vestingAcc, err := sdk.AccAddressFromBech32("pasg105488mw9t3qtp62jhllde28v40xqxpjksjqmvx")
	if err != nil {
		return err
	}

	// 3 year lock-up from relaunch and thereafter weekly vesting until end of year 5 from relaunch
	pva := vestingtypes.NewPeriodicVestingAccount(authtypes.NewBaseAccount(vestingAcc, nil, ak.GetNextAccountNumber(ctx), 0),
		amount,
		1755097200,
		genVestingPeriods(),
	)
	ak.SetAccount(ctx, pva)

	return dk.DistributeFromFeePool(ctx, amount, vestingAcc)
}

func genVestingPeriods() []vestingtypes.Period {
	var periods []vestingtypes.Period
	periods = append(periods, vestingtypes.Period{
		Length: 0,
		Amount: sdk.NewCoins(sdk.NewCoin(upasgDenom, sdk.NewInt(1442307692379))),
	})

	for i := 0; i < 103; i++ {
		periods = append(periods, vestingtypes.Period{
			Length: 604800,
			Amount: sdk.NewCoins(sdk.NewCoin(upasgDenom, sdk.NewInt(1442307692307))),
		})
	}

	return periods
}
