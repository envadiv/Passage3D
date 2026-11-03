package claim_test

import (
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/x/claim"
	"github.com/envadiv/Passage3D/x/claim/types"
)

// airdropEnd is the first instant after the claim window of testGenesis().
var airdropEnd = now.Add(types.DefaultDurationUntilDecay + types.DefaultDurationOfDecay + time.Second)

func communityPool(t *testing.T, ctx sdk.Context, a *app.PassageApp) sdk.DecCoins {
	t.Helper()
	feePool, err := a.DistrKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	return feePool.CommunityPool
}

// claimModuleTransfers returns the coin_spent amounts and the transfer
// recipient/amount pairs whose spender/sender is the claim module account.
func claimModuleTransfers(events []abci.Event, moduleAddr string) (spent, transfers []string) {
	for _, ev := range events {
		attrs := make(map[string]string, len(ev.Attributes))
		for _, attr := range ev.Attributes {
			attrs[attr.Key] = attr.Value
		}
		switch {
		case ev.Type == banktypes.EventTypeCoinSpent && attrs[banktypes.AttributeKeySpender] == moduleAddr:
			spent = append(spent, attrs[sdk.AttributeKeyAmount])
		case ev.Type == banktypes.EventTypeTransfer && attrs[sdk.AttributeKeySender] == moduleAddr:
			transfers = append(transfers, attrs[banktypes.AttributeKeyRecipient]+"/"+attrs[sdk.AttributeKeyAmount])
		}
	}
	return spent, transfers
}

func TestEndBlockerEndsAirdropOnce(t *testing.T) {
	a := app.Setup(false, t)
	ctx := deliverCtx(a, now)
	claim.InitGenesis(ctx, a.ClaimKeeper, testGenesis())

	balance := a.ClaimKeeper.GetModuleAccountBalance(ctx)
	require.True(t, balance.IsPositive())
	moduleAddr := a.ClaimKeeper.GetModuleAccountAddress(ctx).String()
	distrAddr := a.AccountKeeper.GetModuleAddress(distrtypes.ModuleName).String()

	// Inside the claim window EndBlocker does nothing.
	ctx = ctx.WithBlockTime(airdropEnd.Add(-2 * time.Second)).WithEventManager(sdk.NewEventManager())
	claim.EndBlocker(ctx, a.ClaimKeeper)
	require.Empty(t, ctx.EventManager().Events())
	require.True(t, a.ClaimKeeper.GetParams(ctx).AirdropEnabled)
	require.Len(t, a.ClaimKeeper.GetClaimRecords(ctx), 2)

	// First block after the window: the remaining balance goes to the
	// community pool, claim records are removed and the airdrop is disabled.
	poolBefore := communityPool(t, ctx, a)
	ctx = ctx.WithBlockTime(airdropEnd).WithEventManager(sdk.NewEventManager())
	claim.EndBlocker(ctx, a.ClaimKeeper)

	spent, transfers := claimModuleTransfers(ctx.EventManager().ABCIEvents(), moduleAddr)
	require.Equal(t, []string{balance.String()}, spent)
	require.Equal(t, []string{distrAddr + "/" + balance.String()}, transfers)
	require.Equal(t, poolBefore.Add(sdk.NewDecCoinsFromCoins(balance)...).String(), communityPool(t, ctx, a).String())
	require.True(t, a.ClaimKeeper.GetModuleAccountBalance(ctx).IsZero())
	require.Empty(t, a.ClaimKeeper.GetClaimRecords(ctx))
	params := a.ClaimKeeper.GetParams(ctx)
	require.False(t, params.AirdropEnabled)
	require.False(t, params.IsAirdropEnabled(ctx.BlockTime()))

	// Later blocks must not end the airdrop again.
	poolAfter := communityPool(t, ctx, a)
	for i := 1; i <= 3; i++ {
		ctx = ctx.WithBlockTime(airdropEnd.Add(time.Duration(i) * time.Hour)).WithEventManager(sdk.NewEventManager())
		require.NotPanics(t, func() { claim.EndBlocker(ctx, a.ClaimKeeper) })
		require.Empty(t, ctx.EventManager().Events(), "block %d after the airdrop end", i)
	}
	require.Equal(t, poolAfter.String(), communityPool(t, ctx, a).String())
}

// With nothing left in the module account EndAirdrop must not try to fund the
// community pool, but it still disables the airdrop.
func TestEndBlockerEndsAirdropWithEmptyModuleAccount(t *testing.T) {
	a := app.Setup(false, t)
	ctx := deliverCtx(a, now)
	genesis := testGenesis()
	genesis.ModuleAccountBalance = sdk.NewInt64Coin(types.DefaultClaimDenom, 0)
	genesis.ClaimRecords = nil
	claim.InitGenesis(ctx, a.ClaimKeeper, genesis)
	require.True(t, a.ClaimKeeper.GetModuleAccountBalance(ctx).IsZero())

	poolBefore := communityPool(t, ctx, a)
	for i := 0; i < 3; i++ {
		ctx = ctx.WithBlockTime(airdropEnd.Add(time.Duration(i) * time.Hour)).WithEventManager(sdk.NewEventManager())
		require.NotPanics(t, func() { claim.EndBlocker(ctx, a.ClaimKeeper) })
		require.Empty(t, ctx.EventManager().Events())
		require.False(t, a.ClaimKeeper.GetParams(ctx).AirdropEnabled)
	}
	require.Equal(t, poolBefore.String(), communityPool(t, ctx, a).String())
}

// TestEndAirdropAcrossBlocks runs the same scenario through FinalizeBlock and
// Commit, i.e. with the module manager's EndBlock wiring and real block events.
func TestEndAirdropAcrossBlocks(t *testing.T) {
	a := app.Setup(false, t)
	ctx := deliverCtx(a, now)
	claim.InitGenesis(ctx, a.ClaimKeeper, testGenesis())
	balance := a.ClaimKeeper.GetModuleAccountBalance(ctx)
	require.True(t, balance.IsPositive())
	moduleAddr := a.ClaimKeeper.GetModuleAccountAddress(ctx).String()
	distrAddr := a.AccountKeeper.GetModuleAddress(distrtypes.ModuleName).String()

	blockTimes := []time.Time{
		now.Add(time.Minute), // inside the window
		airdropEnd,           // the airdrop ends here
		airdropEnd.Add(time.Minute),
		airdropEnd.Add(time.Hour),
	}
	var spentPerBlock, transfersPerBlock [][]string
	for _, blockTime := range blockTimes {
		res := finalizeAndCommit(t, a, blockTime)
		spent, transfers := claimModuleTransfers(res.Events, moduleAddr)
		spentPerBlock = append(spentPerBlock, spent)
		transfersPerBlock = append(transfersPerBlock, transfers)
	}
	require.Equal(t, [][]string{nil, {balance.String()}, nil, nil}, spentPerBlock)
	require.Equal(t, [][]string{nil, {distrAddr + "/" + balance.String()}, nil, nil}, transfersPerBlock)

	ctx = a.BaseApp.NewContext(true)
	require.False(t, a.ClaimKeeper.GetParams(ctx).AirdropEnabled)
	require.Empty(t, a.ClaimKeeper.GetClaimRecords(ctx))
	require.True(t, a.ClaimKeeper.GetModuleAccountBalance(ctx).IsZero())
}

// finalizeAndCommit executes the next block with the given time and
// transactions and commits it.
func finalizeAndCommit(t *testing.T, a *app.PassageApp, blockTime time.Time, txs ...[]byte) *abci.ResponseFinalizeBlock {
	t.Helper()
	res, err := a.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: a.LastBlockHeight() + 1,
		Time:   blockTime,
		Txs:    txs,
	})
	require.NoError(t, err)
	_, err = a.Commit()
	require.NoError(t, err)
	return res
}
