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
	_ bank.Keeper,
	_ auth.AccountKeeper,
	ck claim.Keeper,
) upgradetypes.UpgradeHandler {

	return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		if err := ExecuteProposal(ctx, ck); err != nil {
			return nil, err
		}

		return fromVM, nil
	}
}

func ExecuteProposal(ctx sdk.Context, ck claim.Keeper) error {
	var newClaimRecords = []claimtypes.ClaimRecord{}
	var sixMonths = time.Hour * 24 * 180

	if err := ck.SetClaimRecords(ctx, newClaimRecords); err != nil {
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
