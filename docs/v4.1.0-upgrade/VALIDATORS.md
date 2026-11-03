# passage-2 upgrade to v4.1.0: validator instructions

| | |
|---|---|
| Upgrade name | `v4.1.0` |
| Height | `PENDING_HEIGHT` (estimated `PENDING_DATE` UTC; follow the height, not the date) |
| Proposal | `PENDING_PROPOSAL_ID` |
| Release | https://github.com/envadiv/Passage3D/releases/tag/v4.1.0 |
| Required | every validator switches binaries at the upgrade height |

## Why

- **Security (critical):** wasmd v0.50.0 -> v0.54.10 and wasmvm v1.5.0 -> v2.2.9 patch
  CWA-2026-006, a Wasmer Singlepass sandbox escape that can lead to native token minting,
  and CWA-2026-005. Restricting code uploads does not fix code that is already stored.
- **Fixes since v4.0.0:** Ledger / Amino-signed CosmWasm transactions accepted again;
  `MsgClaim` accepted again; legacy governance proposals restored; the duplicate account
  number of the claim module account repaired by the upgrade handler.
- Cosmos SDK v0.50.15, ibc-go v8.8.0 (ibc module migration 5 -> 6). Contract code and state
  are preserved. Details: `docs/security-v4.1.0.md`.

## What changes for operators

- **The binary needs its library.** `passage` loads `libwasmvm.x86_64.so`
  (`libwasmvm.aarch64.so` on arm64) from **its own directory** (rpath `$ORIGIN`). Keep both
  files together; the binary does not start without the library next to it.
- Built on Ubuntu 22.04 (glibc 2.35): run it on a distribution with glibc >= 2.35.
- `minimum-gas-prices` in `app.toml` must be at least `12.5upasg` (the node applies that floor).
- New nodes write a `[wasm]` section in `app.toml`; existing files keep working without it.

## Before the upgrade height

1. Download and verify the bundle for your architecture:
   ```sh
   V=4.1.0; ARCH=amd64     # or arm64
   wget https://github.com/envadiv/Passage3D/releases/download/v$V/passage-$V-linux-$ARCH.tar.gz \
        https://github.com/envadiv/Passage3D/releases/download/v$V/SHA256SUMS.txt
   grep "passage-$V-linux-$ARCH" SHA256SUMS.txt | sha256sum -c --ignore-missing
   mkdir -p /tmp/passage-$V && tar -xzf passage-$V-linux-$ARCH.tar.gz -C /tmp/passage-$V
   (cd /tmp/passage-$V && sha256sum -c SHA256SUMS-$ARCH.txt)
   ```
   Compare the checksums with the ones in the proposal `info`.
2. Check the binary and its library:
   ```sh
   /tmp/passage-$V/passage-$V-linux-$ARCH version          # 4.1.0
   /tmp/passage-$V/passage-$V-linux-$ARCH query wasm libwasmvm-version   # 2.2.9
   ```
3. Stage it. With Cosmovisor, copy **both** files into the upgrade directory, named as below:
   ```sh
   D=$DAEMON_HOME/cosmovisor/upgrades/v4.1.0/bin
   mkdir -p $D
   cp /tmp/passage-$V/passage-$V-linux-$ARCH $D/passage
   cp /tmp/passage-$V/libwasmvm.*.so $D/
   $D/passage version && $D/passage query wasm libwasmvm-version
   ```
   The directory name must be exactly `v4.1.0`. Stage the binary manually: the auto-download
   archive has the binary at its root, not under `bin/`.
   Without Cosmovisor, keep both files ready and swap them in after the node halts.
4. Keep the v4.0.3 binary and make sure `minimum-gas-prices >= 12.5upasg`.

## At the upgrade height

- v4.0.3 halts with `UPGRADE "v4.1.0" NEEDED at height: PENDING_HEIGHT`.
- **Back up before starting v4.1.0** (needed for any rollback): with the node stopped,
  copy `data/` and `wasm/` (`cp -a --reflink=auto` or an LVM/ZFS/btrfs snapshot keeps it quick).
  Back up `priv_validator_key.json` and `priv_validator_state.json` as well, and never run two
  instances of the same validator.
- Start v4.1.0 (Cosmovisor switches automatically). The first blocks can take longer while
  the new VM compiles the stored contracts again.

## After the upgrade

```sh
passage version                               # 4.1.0
passage query wasm libwasmvm-version          # 2.2.9
passage q upgrade applied v4.1.0 --node <rpc> # applied height
passage status | jq '.sync_info'              # catching_up false, height increasing
```

Then confirm signing (block explorer or `passage q slashing signing-info`), contract queries
and IBC relaying.

## Rollback

Once v4.1.0 has committed blocks, reverting the binary alone is not safe. A rollback is
coordinated: all validators stop, restore the pre-upgrade backup and restart v4.0.3 with
`--unsafe-skip-upgrades PENDING_HEIGHT`, after the team announces it. Do not do it alone.

## Contact

`PENDING_CONTACT` (validator channel). Report the node version, the last height and the
last 100 log lines.
