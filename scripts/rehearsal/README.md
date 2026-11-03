# Upgrade rehearsal on a copy of passage-2 state

`fork_upgrade.sh` runs the next upgrade on a copy of a real node's data, on the server
that already holds it: nothing is uploaded or downloaded except the two binaries.

## What it does

1. Copies `config/`, `data/` and `wasm/` of a **stopped** node (or of a filesystem snapshot)
   into `WORK` with `cp --reflink=auto` (no extra space on XFS/btrfs with reflink, a full
   copy otherwise). The source is never written.
2. Isolates the copy: new `priv_validator_key.json` and `node_key.json`, no peers or seeds,
   PEX off, every port on 127.0.0.1 and shifted by `PORT_OFFSET` (a live node on the same
   host keeps running).
3. `passage in-place-testnet` (old binary) takes the chain over with the new key: one local
   validator with all the voting power, new chain-id, accounts funded, and the upgrade plan
   scheduled `DELAY` blocks ahead. The old binary halts at that height as on mainnet.
4. Snapshot (code hashes, sha256 of the raw state of every contract, module versions,
   claim module account number, owner of account number 0), then the new binary runs the
   upgrade handler.
5. Checks: blocks after the upgrade, handler applied, code hashes and contract state
   unchanged, module version changes listed, duplicate account number repaired,
   libwasmvm version, a bank send, smart queries on every contract, restart.

## Binaries

- `OLD_BIN`: a binary of the running mainnet version with the `in-place-testnet` command. From v4.1.0
  on, the release binary has it. For the v4.0.3 -> v4.1.0 rehearsal use the
  branch `rehearsal/v4.0.3-in-place-testnet` (v4.0.3 `eea1f3e` + `app/testnet.go` and
  `cmd/passage/cmd/testnet_inplace.go`; consensus code unchanged, the command never runs on `start`).
- `NEW_BIN`: the candidate: `v4.0.x` of envadiv/Passage3D (v4.1.0, PR #217).

Built on the server (Go and gcc installed):

```sh
git clone https://github.com/ECHOAD/Passage3D && cd Passage3D
git checkout rehearsal/v4.0.3-in-place-testnet
GOTOOLCHAIN=go1.22.11 go build -tags "netgo ledger" -o ~/rehearsal/passage-old ./cmd/passage
git fetch https://github.com/envadiv/Passage3D v4.0.x && git checkout FETCH_HEAD
GOTOOLCHAIN=go1.25.14 go build -tags "netgo ledger" -o ~/rehearsal/passage-new ./cmd/passage
```

## Run

```sh
sudo systemctl stop passaged            # or take an LVM/ZFS/btrfs snapshot and point SRC_HOME at it
SRC_HOME=$HOME/.passage WORK=/data/fork \
OLD_BIN=$HOME/rehearsal/passage-old NEW_BIN=$HOME/rehearsal/passage-new \
bash scripts/rehearsal/fork_upgrade.sh
sudo systemctl start passaged           # right after the copy step if WORK is a reflink copy
```

Results: `$WORK/out/summary.txt`, logs `old.log` / `new.log`, snapshots `before/` and `after/`.
Delete `$WORK` afterwards.

Never run it with a validator's real key in the copy (the script replaces it) and never
connect the copy to mainnet peers (the script removes them).
