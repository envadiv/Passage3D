# Passage v4.0.0 software-upgrade proposal — submit guide

**Upgrade name:** `v4.0.0` (must match `app/upgrades/v4.0.0` `const Name`)
**Height:** `19950687` (~2026-07-09 12:00 UTC @ 6.05s blocks; recompute near submit if block time drifts)
**Voting:** 7 days → **submit by 2026-07-02 12:00 UTC**
**Deposit:** 100,000 PASG (`100000000000upasg`, = min deposit; what prop #22 used)
**Format/precedent:** prop #22 (v3.0.0, PASSED). Prop #21 was REJECTED for bare `3.0.0` — the `v` prefix is mandatory.

## Before submit — fill placeholders in proposal_v4.0.0.json
1. `PENDING_AMD64` / `PENDING_ARM64` → real sha256 of the published v4.0.0 release binaries.
2. ~~fork-test~~ DONE: 33/33 codes, 79/79 contracts preserved on real state.
3. Re-verify `height` against current chain height the day of submit.

## Submit (gov v1, on a node with the signing key)
```
passage tx gov submit-proposal docs/v4.0.0-upgrade/proposal_v4.0.0.json \
  --from <key> --chain-id passage-2 \
  --gas auto --gas-adjustment 1.5 --gas-prices 12.5upasg \
  --node <rpc> -y
```
Then deposit (if not met) + vote. Verify on-chain plan name reads exactly `v4.0.0`.
