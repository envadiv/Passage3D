package v2_6

import (
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authz "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bank "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distribution "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	feegrant "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	gov "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
)

const Name = "v2.6.0"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          Name,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{},
}

func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	_ codec.Codec,
	dk distribution.Keeper,
	bk bank.Keeper,
	ak auth.AccountKeeper,
	_ staking.Keeper,
	_ gov.Keeper,
	_ authz.Keeper,
	_ feegrant.Keeper,
	_ claim.Keeper,
	_ consensusparamkeeper.Keeper,
	_ paramskeeper.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		if err := ExecuteProposal(ctx, ak, bk, dk); err != nil {
			return nil, err
		}

		return fromVM, nil
	}
}

func ExecuteProposal(ctx sdk.Context, ak auth.AccountKeeper, bk bank.Keeper, dk distribution.Keeper) error {
	// get airdrop account
	airdropAccAddr, err := sdk.AccAddressFromBech32("pasg1cn5rqy7psjm7h60y8524afvffuktllxh947tx0")
	if err != nil {
		return err
	}

	// unclaimed bonus airdrop amount sent to community pool
	unclaimedAmtInPool := sdk.NewCoins(sdk.NewCoin("upasg", sdk.NewInt(3167829000000)))

	// distribute unclaimed tokens in community pool to airdrop account
	if err := dk.DistributeFromFeePool(ctx, unclaimedAmtInPool, airdropAccAddr); err != nil {
		return err
	}

	// total unclaimed amount from all airdrops, includes unclaimed amount in community pool
	totalUnclaimedAmount := sdk.NewCoins(sdk.NewCoin("upasg", sdk.NewInt(28939737000000)))

	// send total unclaimed amount to the gov module account to burn
	if err := bk.SendCoinsFromAccountToModule(ctx, airdropAccAddr, govtypes.ModuleName, totalUnclaimedAmount); err != nil {
		return err
	}

	// burn total unclaimed amount from the gov module account
	err = bk.BurnCoins(ctx, govtypes.ModuleName, totalUnclaimedAmount)
	if err != nil {
		return err
	}

	return nil
}
