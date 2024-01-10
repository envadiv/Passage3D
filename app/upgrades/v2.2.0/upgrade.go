package v2

import (
	"fmt"
	"time"

	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
	claimtypes "github.com/envadiv/Passage3D/x/claim/types"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

const (
	Name       = "v2.2.0"
	upasgDenom = "upasg"
)

// 150,000,000 $PASG tokens
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
	ck claim.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		if err := ExecuteProposal(ctx, ak, bk, ck, dk); err != nil {
			return nil, err
		}

		return fromVM, nil
	}
}

func ExecuteProposal(ctx sdk.Context, ak auth.AccountKeeper, bk bank.Keeper, ck claim.Keeper, dk distribution.Keeper) error {
	sixMonths := time.Hour * 24 * 180

	vestingAcc, err := sdk.AccAddressFromBech32("pasg105488mw9t3qtp62jhllde28v40xqxpjksjqmvx")
	if err != nil {
		return err
	}

	// 3 year lock-up from relaunch and thereafter weekly vesting until end of year 5 from relaunch
	pva := vestingtypes.NewPeriodicVestingAccount(authtypes.NewBaseAccount(vestingAcc, nil, ak.GetNextAccountNumber(ctx), 0),
		amount,
		1784905200,
		genVestingPeriods(),
	)
	ak.SetAccount(ctx, pva)

	if err := dk.DistributeFromFeePool(ctx, amount, vestingAcc); err != nil {
		return err
	}

	// first clear old data
	ck.ClearInitialClaimables(ctx)

	// add/update the newly added claim records and update module account balance
	var amount sdk.Coins
	for _, record := range NewClaimRecords {
		amount = amount.Add(record.ClaimableAmount...)

		// update the claim record in claim module
		if err := ck.UpdateClaimRecord(ctx, *record); err != nil {
			return err
		}
	}
	ctx.Logger().Info(fmt.Sprintf("added new claim records: %d", len(NewClaimRecords)))

	// get airdrop account
	airdropAccAddr, err := sdk.AccAddressFromBech32("pasg1lel0s624jr9zsz4ml6yv9e5r4uzukfs7hwh22w")
	if err != nil {
		return err
	}

	oldAmount := sdk.Coins{
		sdk.NewCoin(amount[0].Denom, sdk.NewInt(18946800000000)),
	}

	amount = amount.Sub(oldAmount)

	// send the added balances from airdrop account to claim module account
	if err := bk.SendCoinsFromAccountToModule(ctx, airdropAccAddr, claimtypes.ModuleName, amount); err != nil {
		return err
	}
	ctx.Logger().Info(fmt.Sprintf("sent coins: %s from airdrop account to claim module account", amount.String()))

	params := ck.GetParams(ctx)
	params.AirdropEnabled = true
	params.AirdropStartTime = time.Date(2023, 10, 23, 15, 0, 0, 0, time.UTC) // (dd/mm/yyyy: 23/10/2023, 15:00UTC)
	params.DurationOfDecay = sixMonths
	params.DurationUntilDecay = sixMonths

	ck.SetParams(ctx, params)
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
