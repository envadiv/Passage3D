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
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/envadiv/Passage3D/app/upgrades"
)

const Name = "v1.0"
const upasgDenom = "upasg"

var amount = sdk.NewCoins(sdk.NewCoin(upasgDenom, sdk.NewInt(150000000000000)))
var vestingAcc, _ = sdk.AccAddressFromBech32("pasg1wkcml9mjpyu9kzy4fqy322p958809r64mfug82") // TODO: update with actual account

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
		ExecuteProposal1(ctx, dk, bk, ak)
		return fromVM, nil
	}
}

// ExecuteProposal1 moves community pool funds to a vesting account
func ExecuteProposal1(ctx sdk.Context, dk distribution.Keeper, bk bank.Keeper, ak auth.AccountKeeper) error {

	pva := vestingtypes.NewPeriodicVestingAccount(authtypes.NewBaseAccount(vestingAcc, nil, ak.GetNextAccountNumber(ctx), 0),
		amount,
		1755097200,
		genVestingPeriods(),
	)
	ak.SetAccount(ctx, pva)

	feePool := dk.GetFeePool(ctx)
	newPool, negative := feePool.CommunityPool.SafeSub(sdk.NewDecCoinsFromCoins(amount...))
	if negative {
		panic("negative amount")
	}

	feePool.CommunityPool = newPool

	err := bk.SendCoinsFromModuleToAccount(ctx, distributiontypes.ModuleName, vestingAcc, amount)
	if err != nil {
		return err
	}

	dk.SetFeePool(ctx, feePool)

	return nil
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
