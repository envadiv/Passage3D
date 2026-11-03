# Passage v4.1.0 software-upgrade proposal: submit guide

**Upgrade name:** `v4.1.0` (must match `app/upgrades/v4.1.0` `const Name` and the Cosmovisor directory)
**Message:** `/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade` (gov v1). Do **not** wrap it in
`MsgExecLegacyContent`: v4.0.3, the running version, has no legacy governance router.
**Authority:** `pasg10d07y265gmmuvt4z0w9aw880jnsr700je62lmv` (x/gov module account).

passage-2 governance (queried 2026-11-03):

| | Normal | Expedited |
|---|---|---|
| Voting period | 7 days | 24 hours |
| Minimum deposit | 100,000 PASG (`100000000000upasg`) | 1,000,000 PASG (`1000000000000upasg`) |
| Threshold | 50% | 67% |
| Quorum | 50% | 50% |

Average block time: ~5.77 s (10,000 blocks up to height 21,704,696).

## Before submitting

1. The release commit on `v4.0.x` passes CI (unit tests, sims, in-place upgrade, security
   upgrade rehearsal on amd64 and arm64).
2. Mainnet-state rehearsal done (`scripts/rehearsal/fork_upgrade.sh`, see `scripts/rehearsal/README.md`;
   run on a server that holds a passage-2 node).
3. Tag `v4.1.0` pushed; `release.yml` created the draft release. Review it and publish it.
   It contains, per architecture: `passage-4.1.0-linux-<arch>`, `libwasmvm.<x86_64|aarch64>.so`,
   `passage-4.1.0-linux-<arch>.tar.gz` (binary + library + provenance + checksums) and
   `SHA256SUMS.txt`.
4. Fill `proposal_v4.1.0.json`:
   - `PENDING_SHA256_AMD64_TAR_GZ` / `PENDING_SHA256_ARM64_TAR_GZ`: `sha256sum passage-4.1.0-linux-*.tar.gz`
     of the published assets;
   - `PENDING_HEIGHT` (twice: `plan.height` and `summary`), computed the day of submission:
     ```sh
     H=$(passage status --node <rpc> | jq -r '.sync_info.latest_block_height')
     # voting period + ~1 day of margin, at 5.77 s per block
     echo $(( H + (7 + 1) * 86400 * 100 / 577 ))     # normal: about 119,800 blocks
     echo $(( H + (1 + 1) * 86400 * 100 / 577 ))     # expedited: about 29,950 blocks
     ```
     Check the resulting date against the block time of the last days and announce it in UTC.
5. For an expedited proposal set `"expedited": true` and the deposit to `1000000000000upasg`.
   CWA-2026-006 is critical, which supports the expedited path if the deposit and the 67%
   threshold are achievable; otherwise use the normal 7 days.

## Submit (on a node with the signing key)

```sh
passage tx gov submit-proposal docs/v4.1.0-upgrade/proposal_v4.1.0.json \
  --from <key> --chain-id passage-2 \
  --gas auto --gas-adjustment 1.5 --gas-prices 12.5upasg --node <rpc> -y
```

Then deposit (if the minimum is not met) and vote:

```sh
passage q gov proposals --node <rpc> -o json | jq '.proposals[-1] | {id, status, title}'
passage tx gov vote <id> yes --from <key> --chain-id passage-2 --gas-prices 12.5upasg --node <rpc> -y
```

After the proposal passes, the plan must read exactly `v4.1.0` at the announced height:

```sh
passage q upgrade plan --node <rpc>
```

Share `VALIDATORS.md` with the validators as soon as the proposal is submitted.
