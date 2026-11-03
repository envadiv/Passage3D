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
  `MsgClaim` accepted again; legacy governance proposals restored; duplicate account numbers
  repaired by the upgrade handler (see [Accounts that get a new account number](#accounts-that-get-a-new-account-number)).
- Cosmos SDK v0.50.15, ibc-go v8.8.0 (ibc module migration 5 -> 6). Contract code and state
  are preserved. Details: `docs/security-v4.1.0.md`.
- Rehearsed on a copy of passage-2 state (height 21712866, 33 codes, 79 contracts): every
  check passes. Report: [`REHEARSAL.md`](REHEARSAL.md).

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
   /tmp/passage-$V/passage-$V-linux-$ARCH version          # v4.1.0
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

### Build from source instead

You need git, make, gcc and Go 1.23.2 or newer (the release is built with Go 1.25.14). The Go
module and build caches take about 3 GB.

```sh
git clone https://github.com/envadiv/Passage3D passage-4.1.0
cd passage-4.1.0
git checkout v4.1.0
git rev-parse HEAD     # e5cb0c5554e15ddfb7e60297406ed14756e45026
make build             # writes build/passage
./build/passage version --long | grep -E '^(version|commit):'
# version: 4.1.0
# commit: e5cb0c5554e15ddfb7e60297406ed14756e45026
./build/passage query wasm libwasmvm-version   # 2.2.9
```

- `make build` writes only `build/passage`. Do not run `make install` before the halt: it
  replaces `$GOPATH/bin/passage`, which may be the binary your node runs.
- This binary loads libwasmvm 2.2.9 from the Go module cache of the user who built it
  (`$GOMODCACHE/github.com/!cosm!wasm/wasmvm/v2@v2.2.9/internal/api/`, the same file as in the
  release). Build it as the user that runs the node and keep that cache. After
  `go clean -modcache`, or for a user who cannot read the cache, the binary fails with
  `error while loading shared libraries: libwasmvm.x86_64.so: cannot open shared object file`.
- It prints version `4.1.0` (the release prints `v4.1.0`) and build tags `netgo,ledger` (the
  release has `netgo`). Neither affects consensus.
- Stage `build/passage` as in step 3 (without Cosmovisor, keep it ready for the halt). Do not
  copy a library.

### Check the release binary against the source (optional)

The release binary is reproducible. This command rebuilds it from the `v4.1.0` tag in Docker,
with the script, the toolchain and the paths of the release workflow (Ubuntu 22.04, Go 1.25.14),
and prints the checksums. They must equal the release checksums. It needs about 4 GB of disk.

```sh
docker run --rm ubuntu:22.04 bash -euc '
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq && apt-get install -y -qq build-essential patchelf curl git ca-certificates >/dev/null
  G=/opt/hostedtoolcache/go/1.25.14/x64; mkdir -p $G
  curl -fsSL https://go.dev/dl/go1.25.14.linux-amd64.tar.gz | tar -xz -C $G --strip-components=1
  export PATH=$G/bin:$PATH HOME=/home/runner GOPATH=/home/runner/go
  mkdir -p /home/runner/work/Passage3D && cd /home/runner/work/Passage3D
  git clone -q https://github.com/envadiv/Passage3D && cd Passage3D && git checkout -q v4.1.0
  VERSION=v4.1.0 bash scripts/build-security-release.sh >/dev/null 2>&1
  cd build/security-release && sha256sum passage-4.1.0-linux-amd64 libwasmvm.x86_64.so'
# cad371406cac00cfe6ef99400d8c179f1812743a90fd2ce52db0987fff213885  passage-4.1.0-linux-amd64
# bfd7157b96028037eb1bdf82b2b7cbf4ebd38a5ab6b1336f0d35c73cf569c783  libwasmvm.x86_64.so
```

Checked for linux/amd64. With other paths (Go install directory, `GOPATH`, clone directory) the
binary works but its checksum can differ.

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
passage version                               # v4.1.0 (4.1.0 if built with make build)
passage query wasm libwasmvm-version          # 2.2.9
passage q upgrade applied v4.1.0 --node <rpc> # applied height
passage status | jq '.sync_info'              # catching_up false, height increasing
```

Then confirm signing (block explorer or `passage q slashing signing-info`), contract queries
and IBC relaying.

## Accounts that get a new account number

On passage-2 six accounts share account number 0 with
`pasg1lel0s624jr9zsz4ml6yv9e5r4uzukfs7hwh22w`, the account that owns that number. The upgrade
handler gives each of them the next free account number; their balances, keys, sequences and
vesting schedules do not change.

| Account | Type |
|---|---|
| `pasg1m5dncvfv7lvpvycr23zja93fecun2kcvpfszyd` | claim module account |
| `pasg1dnn9gvrjr2rxwma5n0mrcdz08xld0mjr56ktxf` | periodic vesting, multisig |
| `pasg1nvqpvagr7r8t3s7ncmqu904wtwhc9uddvjrgzz` | periodic vesting, multisig |
| `pasg1nma4rcj9nly549jjtmnra0x0n6a9dvg3cp0a4y` | periodic vesting, multisig |
| `pasg14ngd35ndr0ajxfw5k79zpt3hatwcanygtds5w7` | periodic vesting, multisig |
| `pasg1cn5rqy7psjm7h60y8524afvffuktllxh947tx0` | periodic vesting (no public key yet) |

Signers of these accounts: signatures made before the upgrade with account number 0, including
partially signed multisig transactions, are invalid after it. Sign again after the upgrade;
with `--offline`, take the account number from `passage q auth account <address>`. Wallets and
`passage tx` without `--offline` read the new number from the chain.

## Rollback

Once v4.1.0 has committed blocks, reverting the binary alone is not safe. A rollback is
coordinated: all validators stop, restore the pre-upgrade backup and restart v4.0.3 with
`--unsafe-skip-upgrades PENDING_HEIGHT`, after the team announces it. Do not do it alone.

## Contact

`PENDING_CONTACT` (validator channel). Report the node version, the last height and the
last 100 log lines.
