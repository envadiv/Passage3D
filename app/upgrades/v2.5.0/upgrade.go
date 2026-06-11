package v2_5

import (
	"fmt"
	"time"

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
	staking "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	passageante "github.com/envadiv/Passage3D/app/ante"
	"github.com/envadiv/Passage3D/app/upgrades"
	claim "github.com/envadiv/Passage3D/x/claim/keeper"
	claimtypes "github.com/envadiv/Passage3D/x/claim/types"
)

const (
	Name = "v2.5.0"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          Name,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{},
}

func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	appCodec codec.Codec,
	_ distribution.Keeper,
	bk bank.Keeper,
	ak auth.AccountKeeper,
	sk staking.Keeper,
	gk gov.Keeper,
	azk authz.Keeper,
	fk feegrant.Keeper,
	ck claim.Keeper,
	_ consensusparamkeeper.Keeper,
	_ paramskeeper.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		if err := ExecuteProposal(ctx, ak, bk, sk, ck); err != nil {
			return nil, err
		}

		// migrate multisig addresses
		if err := MigrateMultisigAddresses(ctx, appCodec, AddressMigrations, bk, ak, sk, gk,
			azk, fk, ck); err != nil {
			return nil, err
		}

		return fromVM, nil
	}
}

func ExecuteProposal(ctx sdk.Context, ak auth.AccountKeeper, bk bank.Keeper, sk staking.Keeper, ck claim.Keeper) error {
	oneMonth := time.Hour * 24 * 30

	// clear old claim records
	ck.ClearInitialClaimables(ctx)

	// add new claim records and update module account balance
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

	// send the added balances from airdrop account to claim module account
	if err := bk.SendCoinsFromAccountToModule(ctx, airdropAccAddr, claimtypes.ModuleName, amount); err != nil {
		return err
	}
	ctx.Logger().Info(fmt.Sprintf("sent coins: %s from airdrop account to claim module account", amount.String()))

	params := ck.GetParams(ctx)
	params.AirdropEnabled = true
	params.AirdropStartTime = time.Date(2025, 4, 24, 16, 30, 0, 0, time.UTC) // (dd/mm/yyyy: 24/04/2025, 16:30UTC)
	params.DurationOfDecay = time.Second * 1
	params.DurationUntilDecay = oneMonth

	ck.SetParams(ctx, params)

	// set minimum commission rate to validators
	return SetValidatorsMinCommissionRate(ctx, sk, passageante.MinCommissionRate)
}
