package v2

import (
	"time"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
	claimtypes "github.com/envadiv/Passage3D/x/claim/types"
)

const Name = "v2.2.0"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          Name,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{},
}

func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	_ distribution.Keeper,
	bk bank.Keeper,
	ak auth.AccountKeeper,
	ck claim.Keeper,
) upgradetypes.UpgradeHandler {

	return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		if err := ExecuteProposal(ctx, ak, bk, ck); err != nil {
			return nil, err
		}

		return fromVM, nil
	}
}

func ExecuteProposal(ctx sdk.Context, ak auth.AccountKeeper, bk bank.Keeper, ck claim.Keeper) error {
	var sixMonths = time.Hour * 24 * 180
	newClaimRecords := getClaimRecords()

	// sum the newly added claim records balance
	var amount sdk.Coins
	for _, record := range newClaimRecords {
		amount.Add(record.ClaimableAmount...)

		// set the claim record in claim module
		ck.SetClaimRecord(ctx, record)
	}

	// get airdrop account
	airdropAccAddr, err := sdk.AccAddressFromBech32("pasg1lel0s624jr9zsz4ml6yv9e5r4uzukfs7hwh22w")
	if err != nil {
		return err
	}

	// send the added balances from airdrop account to claim module account
	if err := bk.SendCoinsFromAccountToModule(ctx, airdropAccAddr, claimtypes.ModuleName, amount); err != nil {
		return err
	}

	params := ck.GetParams(ctx)
	params.AirdropEnabled = true
	params.AirdropStartTime = time.Date(2023, 10, 16, 15, 0, 0, 0, time.UTC) // (dd/mm/yyyy: 16/10/2023, 15:00)
	params.DurationOfDecay = sixMonths
	params.DurationUntilDecay = sixMonths

	ck.SetParams(ctx, params)
	return nil
}
