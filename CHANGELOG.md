<!--
Guiding Principles:

Changelogs are for humans, not machines.
There should be an entry for every single version.
The same types of changes should be grouped.
Versions and sections should be linkable.
The latest version comes first.
The release date of each version is displayed.
Mention whether you follow Semantic Versioning.

Usage:

Change log entries are to be added to the Unreleased section under the
appropriate stanza (see below). Each entry should ideally include a tag and
the Github issue reference in the following format:

* (<tag>) \#<issue-number> message

The issue numbers will later be link-ified during the release process so you do
not have to worry about including a link manually, but you can if you wish.

Types of changes (Stanzas):

"Features" for new features.
"Improvements" for changes in existing functionality.
"Deprecated" for soon-to-be removed features.
"Bug Fixes" for any bug fixes.
"Client Breaking" for breaking CLI commands and REST routes used by end-users.
"API Breaking" for breaking exported APIs used by developers building on SDK.
"State Machine Breaking" for any changes that result in a different AppState
given same genesisState and txList.
Ref: https://keepachangelog.com/en/1.0.0/
-->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## v4.1.0 (consensus-breaking, on-chain upgrade `v4.1.0`)

Base: v4.0.3 (Cosmos SDK v0.50.13, CometBFT v0.38.23, wasmd v0.50.0, wasmvm v1.5.0, ibc-go v8.0.0).
Every validator must switch binaries at the upgrade height. See `docs/security-v4.1.0.md`.

### Security

- CosmWasm: wasmd v0.50.0 -> v0.54.10 and wasmvm v1.5.0 -> v2.2.9, patching CWA-2026-006 (Wasmer
  Singlepass sandbox escape) and CWA-2026-005 (gas accounting). Cosmos SDK v0.50.15 and ibc-go v8.8.0
  follow wasmd's requirements. Built with Go 1.25.14 (`go 1.23.2` in `go.mod`).

### State machine

- CosmWasm: Amino JSON sign bytes match the Amino clients again (CosmWasm/wasmd#1844 and #1860, part
  of wasmd since v0.51); Ledger / Keplr / Leap Amino / CosmJS Amino-signed wasm transactions are accepted.
- x/gov: legacy (v1beta1) proposal router restored (text, x/params `ParameterChangeProposal`).
- x/claim: `MsgClaim` has the `cosmos.msg.v1.signer` and `amino.name` ("claim/Claim") annotations
  (claim txs were rejected since v4.0.0); unknown claim actions rejected; the module account is
  created with a unique account number; an expired airdrop is disabled instead of `EndAirdrop` on every block.
- Upgrade handler: repairs duplicate account numbers. On passage-2 the claim module account and five
  periodic vesting accounts share account number 0 with its owner; they get new account numbers.
- Ante: wasm `GasRegisterDecorator` (authz exec of gzipped `MsgStoreCode`); the blocked community-pool
  multisig can no longer bypass its restriction by adding a `MsgFundCommunityPool` to the tx;
  CheckTx minimum gas price floor (12.5upasg) restored.

### Node and CLI

- `export --for-zero-height` ported to SDK v0.50 (was a panic); `--modules-to-export` honoured.
- iavl-fork `hasVersion`: empty stores no longer report missing versions (rollback failed with
  "version does not exist" and left the node unable to restart; historical queries failed).
- Nodes apply the 12.5upasg minimum gas price floor (warning) when `minimum-gas-prices` is lower;
  new `app.toml` files default to 12.5upasg.
- REST: node service routes (`/cosmos/base/node/v1beta1/config`, `/status`) and swagger registered again.
- CLI: `SIGN_MODE_TEXTUAL`, `config` (confix), `prune`, `snapshots`, `genesis`, `q block(s)`,
  `q block-results`, `tx simulate`; `[wasm]` app.toml section and `--wasm.*` start flags with libwasmvm
  version check; no duplicated module commands; temp directories cleaned; CLI errors printed.
- `passage init` / `passage testnet` produce `upasg` genesis files; testnet uses 5% min commission.
- The error of `RegisterServices` is checked.

### Build, release, tests

- Release: `scripts/build-security-release.sh` builds the binary with the checksum-verified
  libwasmvm v2.2.9 runtime bundle (`.github/workflows/release.yml`, `make release-build`).
- Dockerfile: Go 1.25.14, static musl binary with checksum-verified libwasmvm v2.2.9, Ledger support.
- Makefile: version from git (fallback to commit), fixed build-tags stamp, proto via proto-builder 0.14.0,
  real simulation targets, `localnet-*` on Docker, `test-upgrade`, `test-contracts`.
- Test suite ported to SDK v0.50 (it no longer compiled since v4.0.0); new tests for every fix,
  application simulations (`app/sim_test.go`), in-place upgrade test from v4.0.3
  (`scripts/tests/upgrade_test.sh`), a 4-validator localnet scenario test (`scripts/tests/localnet_test.sh`)
  and a CosmWasm contract regression test (`make test-contracts`): the `passage-contracts` artifacts and
  the passage-2 contract code across the v4.0.3 -> v4.1.0 upgrade, with old/new outcome comparison.
- Upgrade rehearsal with the v4.0.3 release binary on amd64 and arm64 (`.github/workflows/security-upgrade.yml`).
- CI: golangci-lint configuration valid for v1.64 and the linter runs on the whole tree; the protobuf
  breaking check compares against the pull request base; actions updated to their Node 24 releases;
  the invalid `mergify.yml` workflow removed; Dependabot limited to GitHub Actions.
- Removed unused files: `contrib/devtools`, `scripts/test_cover.sh`, the swagger script and config,
  `third_party/proto` (buf provides the well-known types), the LocalPassage README (its content is in
  the README), the v3.1.0 hand-off notes, backup `go.mod` copies and the Cosmos SDK / Regen issue and
  PR templates.

## [v1.1.0](https://github.com/envadiv/Passage3D/releases/tag/v1.1.0) - 2022-10-17

- Update cosmos-sdk to v0.45.9 and tendermint v0.34.21 which includes the patch for IBC issue
- fix lint by @anilCSE in https://github.com/envadiv/Passage3D/pull/79

**Full Changelog**: https://github.com/envadiv/Passage3D/compare/v1.0.0...v1.1.0

## [v1.0.0](https://github.com/envadiv/Passage3D/releases/tag/v1.0.0) - 2022-08-08

Initial Release!
