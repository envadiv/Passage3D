package keeper

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/envadiv/Passage3D/x/claim/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingexported "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Querier is used as Keeper will have duplicate methods if used directly, and gRPC names take precedence over keeper
type Querier struct {
	Keeper
}

var _ types.QueryServer = Keeper{}

// ModuleAccountBalance returns claim module account balance.
func (k Keeper) ModuleAccountBalance(c context.Context, _ *types.QueryModuleAccountBalanceRequest) (*types.QueryModuleAccountBalanceResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	moduleAccBal := sdk.NewCoins(k.GetModuleAccountBalance(ctx))

	return &types.QueryModuleAccountBalanceResponse{ModuleAccountBalance: moduleAccBal}, nil
}

// Params returns params of the claim module.
func (k Keeper) Params(c context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	params := k.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

func (k Keeper) ClaimRecord(goCtx context.Context, req *types.QueryClaimRecordRequest) (*types.QueryClaimRecordResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}

	claimRecord, err := k.GetClaimRecord(ctx, addr)
	return &types.QueryClaimRecordResponse{ClaimRecord: claimRecord}, err
}

func (k Keeper) ClaimableForAction(goCtx context.Context, req *types.QueryClaimableForActionRequest) (*types.QueryClaimableForActionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}

	action, ok := types.Action_value[req.Action]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid action type: %s", req.Action))
	}

	claimableAmountForAction, err := k.GetClaimableAmountForAction(ctx, addr, action)

	return &types.QueryClaimableForActionResponse{
		Amount: claimableAmountForAction,
	}, err
}

func (k Keeper) TotalClaimable(goCtx context.Context, req *types.QueryTotalClaimableRequest) (*types.QueryTotalClaimableResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}

	coins, err := k.GetUserTotalClaimable(ctx, addr)

	return &types.QueryTotalClaimableResponse{
		Coins: coins,
	}, err
}

func (k Keeper) SupplySummary(goCtx context.Context, req *types.QuerySupplySummaryRequest) (
	*types.QuerySupplySummaryResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	var supplyData types.Supply
	k.bankKeeper.IterateTotalSupply(ctx, func(c sdk.Coin) bool {
		supplyData.Total = append(supplyData.Total, c)
		return false
	})
	bondDenom := k.stakingKeeper.BondDenom(ctx)

	delegationsMap := make(map[string]sdk.Coins)
	k.stakingKeeper.IterateAllDelegations(ctx, func(delegation stakingtypes.Delegation) bool {
		// Converting delegated shares to sdk.Coin
		delegated := sdk.NewCoin(bondDenom, delegation.Shares.TruncateInt())
		delegationsMap[delegation.DelegatorAddress] = delegationsMap[delegation.DelegatorAddress].Add(delegated)
		return false
	})

	k.accountKeeper.IterateAccounts(ctx, func(account authtypes.AccountI) bool {
		if ma, ok := account.(*authtypes.ModuleAccount); ok {
			switch ma.Name {
			case stakingtypes.NotBondedPoolName, stakingtypes.BondedPoolName:
				return false
			}
		}
		delegatedTokens := delegationsMap[account.GetAddress().String()]
		balances := k.bankKeeper.GetAllBalances(ctx, account.GetAddress())
		va, ok := account.(vestingexported.VestingAccount)
		if !ok {
			supplyData.Available.Bonded = supplyData.Available.Bonded.Add(delegatedTokens...)
			supplyData.Available.Unbonded = supplyData.Available.Unbonded.Add(balances...)
		} else {
			vestingCoins := va.GetVestingCoins(ctx.BlockTime())
			delegatedVesting := va.GetDelegatedVesting()
			lockedCoins := va.LockedCoins(ctx.BlockTime())
			spendableCoins := balances.Sub(lockedCoins)
			if delegatedVesting.AmountOf(bondDenom).GT(vestingCoins.AmountOf(bondDenom)) {
				supplyData.Vesting.Bonded = supplyData.Vesting.Bonded.Add(vestingCoins...)
				supplyData.Available.Bonded = supplyData.Available.Bonded.Add(delegatedVesting...).Sub(vestingCoins)
			} else {
				supplyData.Vesting.Bonded = supplyData.Vesting.Bonded.Add(delegatedVesting...)
				supplyData.Available.Bonded = supplyData.Available.Bonded.Add(delegatedTokens...).Sub(delegatedVesting)
			}
			supplyData.Vesting.Unbonded = supplyData.Vesting.Unbonded.Add(lockedCoins...)
			supplyData.Available.Unbonded = supplyData.Available.Unbonded.Add(spendableCoins...)
		}
		return false
	})

	communityPool, _ := k.distrKeeper.GetFeePoolCommunityCoins(ctx).TruncateDecimal()

	supplyData.Circulating = supplyData.Total.Sub(supplyData.Vesting.Unbonded).Sub(supplyData.Vesting.Bonded).Sub(communityPool)

	return &types.QuerySupplySummaryResponse{
		Supply: supplyData,
	}, nil
}
