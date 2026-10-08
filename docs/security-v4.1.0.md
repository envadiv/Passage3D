# Passage v4.1.0 security upgrade candidate

Prepared 2026-10-08. **Candidate only: do not activate on mainnet until a recent
mainnet-state rehearsal and validator coordination are complete.** The proposed
plan name is `v4.1.0`; the candidate binary version is `v4.1.0-rc1`.

## Why this upgrade

The CryptoDungeon node on `passage-2` reports Passage v4.0.3, wasmd v0.50.0,
wasmvm v1.5.0 (also confirmed by the runtime library query), SDK v0.50.13, and
IBC v8.0.0. At inspection it was synced, at height 21,320,724, with no scheduled
upgrade. This is one node's observation, not a survey of every validator.

[CWA-2026-006](https://github.com/CosmWasm/advisories/blob/main/CWAs/CWA-2026-006.md)
discloses a critical Wasmer Singlepass sandbox escape that can result in native
token minting and fund loss. It was publicly patched September 28, 2026.
Unsupported versions are also affected. It requires a coordinated upgrade;
restricting uploads does not fix previously stored malicious code. Passage's
current upload allowlist reduces delivery opportunities but is not a patch.

The candidate also incorporates earlier CosmWasm fixes, including the
consensus-breaking gas accounting changes in
[CWA-2026-005](https://github.com/CosmWasm/advisories/blob/main/CWAs/CWA-2026-005.md).
We do not run exploit contracts on mainnet.

## Dependency target

| Component | Observed baseline | Candidate |
| --- | --- | --- |
| wasmd | 0.50.0 | **0.54.10** |
| wasmvm | 1.5.0 | **2.2.9** |
| Cosmos SDK | 0.50.13 | **0.50.15** |
| IBC | 8.0.0 | **8.8.0** |
| CometBFT | 0.38.23 in binary build metadata | **0.38.23** |
| IAVL | Local Passage fork | Local Passage fork retained |

The SDK/IBC versions follow wasmd 0.54.10's dependency requirements. This is the
closest patched upgrade path, reducing the migration surface compared with
moving SDK and IBC major versions at the same time. It is an interim security
catch-up, not a claim of long-term support for SDK 0.50. A later upgrade to a
maintained SDK/wasmd major line should be separately scoped and rehearsed.

The IAVL replacement is preserved because Passage depends on its empty-store
load fix. The resolved upstream requirement is 1.2.4, but `replace` still points
to the inherited local fork. A module version string alone does not establish
that this fork contains every upstream 1.2.4 change.

## Implementation

- Registers the exact `v4.1.0` upgrade name and runs module migrations.
- Adds, removes, or renames no stores. Does not intentionally change tokenomics,
  fees, upload permissions, staking/governance parameters, or contract state.
- Adapts the Wasm keeper to the new node/VM configuration API, uses the VM's
  built-in capability list, and enables the modern VM's supported capabilities.
- Bundles the **2.2.9** shared library with a pinned SHA256, then verifies
  `passage query wasm libwasmvm-version` reports **2.2.9**.
- Updates the legacy GoReleaser library pins and provides native amd64/arm64
  build jobs. Tag builds produce a draft release for review. The workflow has
  not been executed remotely during preparation of this candidate.

## Build and local rehearsal

On Linux with glibc, install Go 1.25.14 and a native GCC, then run:

```bash
VERSION=v4.1.0-rc1 bash scripts/build-security-release.sh
build/security-release/passage-4.1.0-rc1-linux-amd64 query wasm libwasmvm-version
```

Keep the executable and `libwasmvm.x86_64.so` (or the arm64 library) together in the same directory. The executable uses an origin-relative runtime search path. Unpack and checksum-check both files; Cosmovisor staging must copy both into the upgrade bin directory. The static configuration is retained as a legacy path but is not validated for this candidate.

The build uses `-mod=readonly`. Final versions require a clean committed
checkout. An RC may be built from dirty inputs, but its provenance records that
fact and it must not be presented as a clean release artifact.

The disposable-chain rehearsal requires an old v4.0.3 binary and the original
CosmWasm 1.x `hackatom.wasm` fixture from wasmd v0.50.0:

```bash
python3 scripts/test-security-upgrade.py \
  --old /absolute/path/to/passage-4.0.3-linux-amd64 \
  --new /absolute/path/to/passage-4.1.0-rc1-linux-amd64 \
  --contract /absolute/path/to/hackatom.wasm \
  --work-dir /absolute/path/to/disposable-test-directory
```

It uses only disposable keys and an isolated local chain, with RPC/P2P on
127.0.0.1 ports 28657/28656. It stores and instantiates a contract before a
governance vote, checks that the old binary halts at the scheduled height, starts
the candidate, compares code hashes, contract metadata/state and permissions,
executes the existing contract, broadcasts a bank transfer, checks the applied
upgrade height and verifies restart progress. It reserves account number zero
in the disposable genesis to accommodate the old binary's pre-existing claim
module initialization bug. This is not a modification to mainnet state.

## Remaining mainnet release requirements

1. Obtain a consistent recent **offline snapshot** including application DB,
   Wasm code and chain state. Never copy an actively changing LevelDB and call
   it a validated snapshot. Never use a production signing key for a fork.
2. Rehearse on an isolated fork using the current binary first, then activate
   `v4.1.0` at a scheduled height with the candidate. Bind to loopback and clear
   seeds/peers; prevent connectivity to mainnet.
3. Compare all Wasm code hashes, contract metadata and state, balances/supply,
   delegations, module version maps, permissions and governance/staking params.
   Test representative existing NFT/marketplace contracts, signing modes,
   staking/rewards, IBC client queries and actual IBC transfers against an
   isolated counterparty. Verify restart and state export/import.
4. Resolve the inherited legacy test-suite imports/API incompatibilities before
   describing the full unit suite as passing. The local governance rehearsal
   does not replace that work or the real-state fork.
5. Have the Passage maintainers confirm the release number, fork evidence,
   activation method/height, release commit and both architecture artifacts.
   Coordinate validator readiness before submitting a proposal or binary swap.

No governance transaction, production binary replacement, service restart,
upstream PR, release tag or publication is performed by this candidate build.

## Validator staging after release approval

Use the final maintainer-published artifact and its published checksum, not an
unreviewed RC. Check the binary version and runtime library first. Stage it in
`$DAEMON_HOME/cosmovisor/upgrades/v4.1.0/bin/passage` together with its bundled runtime library; the directory and governance
plan must both use exactly `v4.1.0`. Keep the old binary and backups, and allow
Cosmovisor to switch only at the agreed upgrade height. Do not swap early.

If the patched binary has committed state, reverting the executable alone is
not a safe rollback. Recovery must use the coordinated plan and consistent
pre-upgrade state/signing backups. Confirm block production, applied height,
runtime library 2.2.9, contract queries, staking and IBC after activation.
