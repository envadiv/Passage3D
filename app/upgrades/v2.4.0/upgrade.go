package v2

import (
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
	claimtypes "github.com/envadiv/Passage3D/x/claim/types"
)

const Name = "v2.4.0"

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
	_ claim.Keeper,
) upgradetypes.UpgradeHandler {

	return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		if err := ExecuteProposal(ctx, ak, bk); err != nil {
			return nil, err
		}

		return fromVM, nil
	}
}

func ExecuteProposal(ctx sdk.Context, ak auth.AccountKeeper, bk bank.Keeper) error {
	// get airdrop claim module account
	addr := ak.GetModuleAddress(claimtypes.ModuleName)

	// get all balances of airdrop claim module account
	balances := bk.GetAllBalances(ctx, addr)

	// claim module account do not have the burner permissions
	// so we are sending it to the another module account and burning the tokens
	if err := bk.SendCoinsFromModuleToModule(ctx, claimtypes.ModuleName, govtypes.ModuleName, balances); err != nil {
		return err
	}

	// burn the coins from the module account
	err := bk.BurnCoins(ctx, govtypes.ModuleName, balances)
	if err != nil {
		return err
	}
	return nil
}
