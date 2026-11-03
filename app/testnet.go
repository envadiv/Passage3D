package app

import (
	"fmt"
	"time"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/libs/bytes"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// TestnetOptions configures InitPassageAppForTestnet.
type TestnetOptions struct {
	// NewValAddr and NewValPubKey identify the local priv_validator_key (set by the SDK).
	NewValAddr   bytes.HexBytes
	NewValPubKey crypto.PubKey
	// NewOperator is the bech32 account address that operates the new validator.
	NewOperator string
	// AccountsToFund receive FundAmount each (the operator is always funded).
	AccountsToFund []string
	FundAmount     sdkmath.Int
	// ScheduleUpgrade, when set, schedules that upgrade plan ScheduleUpgradeDelay blocks
	// after the current height, so that this binary halts there and the next binary
	// runs the upgrade handler, as on mainnet after a governance vote.
	ScheduleUpgrade      string
	ScheduleUpgradeDelay int64
}

// testnetValidatorPower is the consensus power given to the local validator.
const testnetValidatorPower = 900_000_000

// InitPassageAppForTestnet turns state loaded from a copy of a passage-2 node into a
// chain controlled by one local validator ("in-place testnet", see `passage
// in-place-testnet`). The SDK replaces the CometBFT validator set; this function
// makes the application state agree with it:
//
//   - staking: the local validator is bonded with all the voting power; every
//     other validator is removed from the power index and the last-validator set
//     (they stay in the store, so delegations and queries keep working);
//   - slashing: signing info for the new consensus address;
//   - bank: the bonded tokens are minted into the bonded pool and the operator
//     (and AccountsToFund) are funded for fees and deposits;
//   - gov: short voting periods and small deposits for manual proposals;
//   - upgrade: optionally schedules ScheduleUpgrade.
//
// It is only reachable through `in-place-testnet` and must never run on a mainnet node.
func InitPassageAppForTestnet(app *PassageApp, opts TestnetOptions) (*PassageApp, error) {
	ctx := app.BaseApp.NewUncachedContext(true, cmtproto.Header{Height: app.LastBlockHeight(), Time: time.Now().UTC()})

	operator, err := sdk.AccAddressFromBech32(opts.NewOperator)
	if err != nil {
		return nil, fmt.Errorf("operator address: %w", err)
	}
	valAddr := sdk.ValAddress(operator)

	// STAKING
	pubKey := &ed25519.PubKey{Key: opts.NewValPubKey.Bytes()}
	pubKeyAny, err := codectypes.NewAnyWithValue(pubKey)
	if err != nil {
		return nil, err
	}
	tokens := sdk.TokensFromConsensusPower(testnetValidatorPower, sdk.DefaultPowerReduction)
	newVal := stakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		ConsensusPubkey: pubKeyAny,
		Jailed:          false,
		Status:          stakingtypes.Bonded,
		Tokens:          tokens,
		DelegatorShares: sdkmath.LegacyNewDecFromInt(tokens),
		Description:     stakingtypes.NewDescription("passage-fork", "", "", "", "in-place testnet validator"),
		Commission: stakingtypes.NewCommission(
			sdkmath.LegacyNewDecWithPrec(5, 2), sdkmath.LegacyNewDecWithPrec(20, 2), sdkmath.LegacyNewDecWithPrec(1, 2),
		),
		MinSelfDelegation: sdkmath.OneInt(),
	}

	stakingStore := ctx.KVStore(app.GetKey(stakingtypes.StoreKey))
	var stale [][]byte
	powerIt, err := app.StakingKeeper.ValidatorsPowerStoreIterator(ctx)
	if err != nil {
		return nil, err
	}
	for ; powerIt.Valid(); powerIt.Next() {
		stale = append(stale, powerIt.Key())
	}
	powerIt.Close()
	lastIt, err := app.StakingKeeper.LastValidatorsIterator(ctx)
	if err != nil {
		return nil, err
	}
	for ; lastIt.Valid(); lastIt.Next() {
		stale = append(stale, lastIt.Key())
	}
	lastIt.Close()
	for _, k := range stale {
		stakingStore.Delete(k)
	}

	if err := app.StakingKeeper.SetValidator(ctx, newVal); err != nil {
		return nil, err
	}
	if err := app.StakingKeeper.SetValidatorByConsAddr(ctx, newVal); err != nil {
		return nil, err
	}
	if err := app.StakingKeeper.SetValidatorByPowerIndex(ctx, newVal); err != nil {
		return nil, err
	}
	if err := app.StakingKeeper.SetLastValidatorPower(ctx, valAddr, 0); err != nil {
		return nil, err
	}
	if err := app.StakingKeeper.Hooks().AfterValidatorCreated(ctx, valAddr); err != nil {
		return nil, err
	}

	// SLASHING
	consAddr := sdk.ConsAddress(opts.NewValAddr.Bytes())
	signingInfo := slashingtypes.NewValidatorSigningInfo(consAddr, app.LastBlockHeight(), 0, time.Unix(0, 0), false, 0)
	if err := app.SlashingKeeper.SetValidatorSigningInfo(ctx, consAddr, signingInfo); err != nil {
		return nil, err
	}

	// BANK: back the new validator's tokens and fund the accounts used on the fork.
	bondDenom, err := app.StakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	bonded := sdk.NewCoins(sdk.NewCoin(bondDenom, tokens))
	if err := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, bonded); err != nil {
		return nil, err
	}
	if err := app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, stakingtypes.BondedPoolName, bonded); err != nil {
		return nil, err
	}
	fund := sdk.NewCoins(sdk.NewCoin(bondDenom, opts.FundAmount))
	for _, a := range append([]string{opts.NewOperator}, opts.AccountsToFund...) {
		addr, err := sdk.AccAddressFromBech32(a)
		if err != nil {
			return nil, fmt.Errorf("account to fund %q: %w", a, err)
		}
		if err := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, fund); err != nil {
			return nil, err
		}
		if err := app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, fund); err != nil {
			return nil, err
		}
	}

	// GOV: short periods and small deposits for proposals made on the fork.
	govParams, err := app.GovKeeper.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	votingPeriod, expeditedPeriod := 60*time.Second, 30*time.Second
	govParams.VotingPeriod = &votingPeriod
	govParams.ExpeditedVotingPeriod = &expeditedPeriod
	govParams.MinDeposit = sdk.NewCoins(sdk.NewInt64Coin(bondDenom, 1_000_000))
	govParams.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewInt64Coin(bondDenom, 2_000_000))
	if err := app.GovKeeper.Params.Set(ctx, govParams); err != nil {
		return nil, err
	}

	// UPGRADE
	if opts.ScheduleUpgrade != "" {
		plan := upgradetypes.Plan{Name: opts.ScheduleUpgrade, Height: app.LastBlockHeight() + opts.ScheduleUpgradeDelay}
		if err := app.UpgradeKeeper.ScheduleUpgrade(ctx, plan); err != nil {
			return nil, fmt.Errorf("scheduling upgrade %s: %w", plan.Name, err)
		}
	}

	return app, nil
}
