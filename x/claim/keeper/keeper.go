package keeper

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/envadiv/Passage3D/x/claim/types"
	"github.com/tendermint/tendermint/libs/log"
)

// Keeper struct
type Keeper struct {
	cdc           codec.Codec
	storeKey      storetypes.StoreKey
	accountKeeper authkeeper.AccountKeeper
	bankKeeper    bankkeeper.Keeper
	stakingKeeper types.StakingKeeper
	distrKeeper   distrkeeper.Keeper
	paramstore    paramtypes.Subspace
}

// NewKeeper returns keeper
func NewKeeper(cdc codec.Codec, storeKey storetypes.StoreKey, ak authkeeper.AccountKeeper, bk bankkeeper.Keeper, sk types.StakingKeeper, dk distrkeeper.Keeper, ps paramtypes.Subspace) Keeper {
	// set KeyTable if it has not already been set
	if !ps.HasKeyTable() {
		ps = ps.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		accountKeeper: ak,
		bankKeeper:    bk,
		stakingKeeper: sk,
		distrKeeper:   dk,
		paramstore:    ps,
	}
}

// Logger returns logger
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
