# Passage v3.1.0 — Cosmos SDK 0.47 → 0.50 upgrade (REVIEW CANDIDATE)

Prepared by CryptoDungeon (Ninjaxan). **This is a candidate for your review and
joint testing — NOT a finished, ship-it binary.** Please read the "Status / what's
left" section before scheduling any on-chain proposal.

## What it is
Migrates Passage (`passage-2`, currently v3.0.0 = SDK 0.47) to **SDK v0.50.13**:
- cosmos-sdk v0.47.13 → **v0.50.13**
- cometbft v0.37 → **v0.38.12**
- ibc-go v7 → **v8.0.0**
- wasmd v0.40 → **v0.50.0** (wasmvm stays **1.5.x** — no contract 1→2 re-validation this hop)
- iavl v0.20 → **v1.2.2 + a required patch (see below)**
- Adds **expedited proposals** automatically (0.50 gov migration; default expedited
  voting period 24h) — gives Passage a fast on-chain emergency-gov lever.

On-chain upgrade name: **`v050`**. Handler = `RunMigrations` (StoreUpgrades empty —
0.50 adds no module stores).

## ⚠️ The one change that needs your scrutiny: a forked iavl patch
SDK 0.50 / iavl v1 cannot load a 0.47 (iavl-0.20) database when any store is
**empty** (e.g. x/evidence, x/feegrant with no entries). iavl's `db.Get`/`db.Has`
treat a zero-length value as absent, and an empty tree stores its root marker with
an empty value — so iavl reports those versions as missing and the node aborts at
startup with `failed to load store: version does not exist`. **Unpatched, every
validator's v0.50 binary would fail to start at the upgrade height = chain halt.**

Fix (`iavl-empty-store-fix.patch`, 2 functions in iavl `nodedb.go`): use a
value-agnostic key-existence check (iterator) for both the legacy `r<version>` and
native `s<version>` root reads. Applied via `replace github.com/cosmos/iavl => ./iavl-fork`.
**This is consensus-critical — please review it, and we propose upstreaming it to
cosmos/iavl as a PR (it affects any 0.47→0.50 chain with empty stores).**

Other notable fixes (all in the branch): x/upgrade wired as a **PreBlocker** (0.50)
so RunMigrations runs before BeginBlock (else ibc client-params panic); 0.50
`BasicModuleManager` + autoCLI in the root cmd (a bare AppModuleBasic{} nil-panics
in 0.50).

## Status / what's left (DO NOT propose until these are green)
PASSED so far (our testing):
- ✅ Synthetic 0.47→0.50 gov-upgrade dry-run: upgrade applies, migrates gov/ibc/
  slashing, blocks continue, full REST query battery passes (incl. historical
  pre-upgrade height), expedited params populated.
- ✅ Real-mainnet-data load: our binary loads all 19 stores from a real
  `passage_19809576` snapshot (incl. the empty stores) — the brick-the-chain class
  validated on real state.

NOT yet done — recommend we do these together before any prop:
- ❌ RunMigrations against full real mainnet state (real IBC clients / gov props / claims).
- ❌ CosmWasm contract execution parity (wasmd 0.40→0.50, wasmvm 1.5).
- ❌ Multi-validator localnet: app-hash agreement across validators + determinism.
- ❌ Upstream/independent review of the iavl patch.

## Build & verify (reproducible)
1. `git fetch <fork> feat/sdk-v0.50-upgrade && git checkout feat/sdk-v0.50-upgrade`
2. Apply the iavl patch into a local `./iavl-fork` (copy of iavl v1.2.2) — the
   go.mod `replace` points there. (Patch: `iavl-empty-store-fix.patch`.)
3. `make build` (go 1.21).
4. Verify: `passage version --long` → `cosmos_sdk_version: v0.50.13`.

Reference binary sha256 (our build): `2cf036c3b2f6504ec1dc84eb3a947bf40ff03e6fc17cdc79beca0f4e45c2d799`

## Upgrade test harness
`upgrade_dryrun_hop2.sh` (synthetic gov-upgrade + REST query battery) is included so
you can reproduce our Tier-1 result and extend it to real state.
