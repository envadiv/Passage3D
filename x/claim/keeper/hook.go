package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/envadiv/Passage3D/x/claim/types"
)

func (k Keeper) AfterProposalVote(ctx sdk.Context, proposalID uint64, voterAddr sdk.AccAddress) {
	_, err := k.ClaimCoinsForAction(ctx, voterAddr, types.ActionVote)
	if err != nil {
		panic(err.Error())
	}
}

func (k Keeper) AfterDelegationModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	// must not run on genesis
	if ctx.BlockHeight() <= 1 {
		return
	}

	_, err := k.ClaimCoinsForAction(ctx, delAddr, types.ActionDelegateStake)
	if err != nil {
		panic(err.Error())
	}
}

// ________________________________________________________________________________________

// Hooks wrapper struct for slashing keeper
type Hooks struct {
	k Keeper
}

var _ govtypes.GovHooks = Hooks{}
var _ stakingtypes.StakingHooks = Hooks{}

// Return the wrapper struct
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// AfterProposalDeposit implements types.GovHooks
func (Hooks) AfterProposalDeposit(ctx sdk.Context, proposalID uint64, depositorAddr sdk.AccAddress) {
}

// AfterProposalFailedMinDeposit implements types.GovHooks
func (Hooks) AfterProposalFailedMinDeposit(ctx sdk.Context, proposalID uint64) {
}

// AfterProposalSubmission implements types.GovHooks
func (Hooks) AfterProposalSubmission(ctx sdk.Context, proposalID uint64) {
}

// AfterProposalVote implements types.GovHooks
func (h Hooks) AfterProposalVote(ctx sdk.Context, proposalID uint64, voterAddr sdk.AccAddress) {
	h.k.AfterProposalVote(ctx, proposalID, voterAddr)
}

// AfterProposalVotingPeriodEnded implements types.GovHooks
func (Hooks) AfterProposalVotingPeriodEnded(ctx sdk.Context, proposalID uint64) {
}

// AfterValidatorBeginUnbonding implements types.StakingHooks
func (Hooks) AfterValidatorBeginUnbonding(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) {
}

// AfterValidatorBonded implements types.StakingHooks
func (Hooks) AfterValidatorBonded(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) {
}

// AfterValidatorCreated implements types.StakingHooks
func (Hooks) AfterValidatorCreated(ctx sdk.Context, valAddr sdk.ValAddress) {
}

// AfterValidatorRemoved implements types.StakingHooks
func (Hooks) AfterValidatorRemoved(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) {
}

// BeforeDelegationCreated implements types.StakingHooks
func (Hooks) BeforeDelegationCreated(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
}

// BeforeDelegationRemoved implements types.StakingHooks
func (Hooks) BeforeDelegationRemoved(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
}

// BeforeDelegationSharesModified implements types.StakingHooks
func (Hooks) BeforeDelegationSharesModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
}

// BeforeValidatorModified implements types.StakingHooks
func (Hooks) BeforeValidatorModified(ctx sdk.Context, valAddr sdk.ValAddress) {
}

// BeforeValidatorSlashed implements types.StakingHooks
func (Hooks) BeforeValidatorSlashed(ctx sdk.Context, valAddr sdk.ValAddress, fraction sdk.Dec) {
}

func (h Hooks) AfterDelegationModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	h.k.AfterDelegationModified(ctx, delAddr, valAddr)
}
