package v4_1_0

import (
	"context"
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	auth "github.com/cosmos/cosmos-sdk/x/auth/keeper"
)

// FixDuplicateAccountNumbers repairs accounts whose account number is not owned by
// them in the x/auth account-number index.
//
// Background: up to v4.0.3 the x/claim module created its module account with
// authtypes.NewEmptyModuleAccount + SetModuleAccount, i.e. with account number 0.
// Under SDK v0.45 that silently produced a second account with number 0 (on
// passage-2 the claim module account shares number 0 with the vesting multisig
// that owns the index entry). SDK v0.50 keeps a *unique* index on account numbers,
// so the state is inconsistent: re-saving the claim account steals index entry 0
// from the multisig (AccountAddressByID(0) then returns the wrong account), and an
// exported genesis gets silently renumbered on import.
//
// For every account whose number is indexed to a different address (a duplicate)
// the account gets a fresh number from the global sequence, and the rightful owner
// is written again so that its index entry is restored (collections removes the
// old index key when the duplicate is re-numbered). Accounts whose index entry is
// simply missing are re-written to recreate it. The function is idempotent.
//
// It returns the bech32 addresses of the accounts that were changed.
func FixDuplicateAccountNumbers(ctx context.Context, ak auth.AccountKeeper) ([]string, error) {
	type entry struct {
		addr sdk.AccAddress
		acc  sdk.AccountI
	}

	// Collect first (no writes while walking the map).
	var candidates []entry
	err := ak.Accounts.Walk(ctx, nil, func(addr sdk.AccAddress, acc sdk.AccountI) (bool, error) {
		owner, err := ak.Accounts.Indexes.Number.MatchExact(ctx, acc.GetAccountNumber())
		switch {
		case errors.Is(err, collections.ErrNotFound):
			candidates = append(candidates, entry{addr, acc}) // number not indexed
		case err != nil:
			return true, err
		case !owner.Equals(addr):
			candidates = append(candidates, entry{addr, acc}) // number indexed to another account
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking accounts: %w", err)
	}

	changed := make([]string, 0, len(candidates))
	for _, e := range candidates {
		// Re-evaluate: earlier iterations may have changed the index.
		oldNumber := e.acc.GetAccountNumber()
		owner, err := ak.Accounts.Indexes.Number.MatchExact(ctx, oldNumber)
		switch {
		case errors.Is(err, collections.ErrNotFound):
			// Nobody owns the number: re-writing the account recreates its index entry.
			ak.SetAccount(ctx, e.acc)
			changed = append(changed, e.addr.String())
			continue
		case err != nil:
			return nil, fmt.Errorf("resolving owner of account number %d: %w", oldNumber, err)
		case owner.Equals(e.addr):
			continue
		}

		newNumber, err := ak.AccountNumber.Next(ctx)
		if err != nil {
			return nil, fmt.Errorf("allocating account number: %w", err)
		}
		if err := e.acc.SetAccountNumber(newNumber); err != nil {
			return nil, err
		}
		// Re-writing the duplicate removes the index key of its old number (which
		// points at the owner) and adds the new number.
		ak.SetAccount(ctx, e.acc)

		// Restore the owner's index entry.
		ownerAcc := ak.GetAccount(ctx, owner)
		if ownerAcc == nil {
			return nil, fmt.Errorf("owner %s of account number %d not found", owner, oldNumber)
		}
		ak.SetAccount(ctx, ownerAcc)

		changed = append(changed, e.addr.String())
	}

	return changed, nil
}
