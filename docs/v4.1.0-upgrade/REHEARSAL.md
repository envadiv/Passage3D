# v4.1.0 upgrade rehearsal on passage-2 state

The v4.0.3 -> v4.1.0 upgrade was run on a copy of a passage-2 node's state with
`scripts/rehearsal/fork_upgrade.sh` (see `scripts/rehearsal/README.md`). The text output is
in [`rehearsal/`](rehearsal/). The full output, logs included, is attached to the
[v4.1.0-rc1 release](https://github.com/envadiv/Passage3D/releases/tag/v4.1.0-rc1) as
`rehearsal-v4.1.0-passage-2.tar.gz` (SHA-256
`0a617b76b381d26937358153f61d6ef53b6eac62a7a11c786c3766ada63f9105`).

**Result: every check passes.** The script printed one `FAIL` (bank send). It is a false
negative of the script; the transaction was included (see [Bank send](#bank-send)).

## Setup

| | |
|---|---|
| Date | 2026-11-03, 17:10 to 17:14 UTC |
| Source | passage-2 full node (not a validator) running v4.0.3, stopped for the copy |
| State | height 21712866: 33 wasm codes, 79 contracts |
| Upgrade height on the copy | 21712886 (plan `v4.1.0`, scheduled 20 blocks ahead) |
| Host | Ubuntu 26.04.1, kernel 7.0.0, glibc 2.43, ext4 (full copy, no reflink) |
| Old binary | v4.0.3 (`eea1f3e`) with the `in-place-testnet` command added (`ef9bfd2`), built on the host |
| New binary | release asset `passage-4.1.0-rc1-linux-amd64` with `libwasmvm.x86_64.so` |
| Script | `scripts/rehearsal/fork_upgrade.sh` at `v4.1.0-rc1` |

The old binary differs from the v4.0.3 release only by `app/testnet.go`,
`cmd/passage/cmd/testnet_inplace.go` and the line in `cmd/passage/cmd/root.go` that registers
the command. The command never runs on `passage start`, so consensus is that of v4.0.3.

| File | SHA-256 | Matches |
|---|---|---|
| `passage-old` | `d4698ba128a1b42eeeb2f0b919e42e7817b2ebf87fc715872186cf74f4e696da` | built on the host |
| `passage-4.1.0-rc1-linux-amd64` | `e35ae07ecf47c52469f0457d224e11e869cba6ce7fa9f7b492d58446d9d36ab4` | `SHA256SUMS.txt` of v4.1.0-rc1 |
| `libwasmvm.x86_64.so` | `bfd7157b96028037eb1bdf82b2b7cbf4ebd38a5ab6b1336f0d35c73cf569c783` | `SHA256SUMS.txt` of v4.1.0-rc1 |
| `fork_upgrade.sh` | `8a64444c11b2616371bf1b05de218a189ebdf20a1e276e541e47eec6ccbe7737` | the file at `v4.1.0-rc1` |

## Results

| Check | Result | Evidence |
|---|---|---|
| Copy isolated: new validator and node keys, no peers, localhost ports | PASS | `summary.txt` |
| v4.0.3 produces blocks on the copy and halts at the upgrade height | PASS | `summary.txt`, `old.log` |
| v4.1.0-rc1 runs the `v4.1.0` handler and produces blocks | PASS | `summary.txt`, `new.log` |
| Wasm code hashes of the 33 codes unchanged | PASS | `before/codes.txt`, `after/codes.txt` |
| Raw state of the 79 contracts unchanged (SHA-256 of `contract-state all`) | PASS | `before/contracts.txt`, `after/contracts.txt` |
| Module versions: only `ibc` changes, 5 -> 6 | expected | `before/module-versions.json`, `after/module-versions.json` |
| Duplicate account numbers repaired | PASS | `accounts.txt`, `new.log` |
| libwasmvm 2.2.9 loaded | PASS | `summary.txt` |
| Bank send included in a block | PASS (script: FAIL, false negative) | `operator-account.json`, `environment.txt` |
| Smart query answered by each of the 79 contracts | PASS | `smart-queries.txt` |
| Restart with the new binary continues | PASS | `summary.txt`, `new2.log` |

The text files are in `rehearsal/`; the logs (`*.log`) are in the release attachment.
73 of the 79 contract state hashes are distinct and none is the hash of an empty result, so
the state comparison covers real data.

## Bank send

The script looks the transaction up by hash (`q tx`). The source node runs with
`indexer = "null"` (`environment.txt`) and the copy inherits that setting, so no
transaction can be found by hash. The broadcast returned `code: 0`, and after the run the
operator account (created on the copy, no other transaction) has `sequence: 1`
(`operator-account.json`): the transaction was included. A later version of the script
checks the sequence instead.

## Account numbers

On passage-2, seven accounts have account number 0. The index entry for number 0 belongs to
`pasg1lel0s624jr9zsz4ml6yv9e5r4uzukfs7hwh22w`, a 3-of-N multisig vesting account. The other six
are duplicates:

- the claim module account `pasg1m5dncvfv7lvpvycr23zja93fecun2kcvpfszyd`;
- five `PeriodicVestingAccount`s. Four of them are 3-of-N multisigs that have signed
  transactions with account number 0 (sequence 12 to 27). The fifth has no public key yet.

The upgrade handler (`FixDuplicateAccountNumbers`) gives each of the six a new number from
the account-number sequence and leaves number 0 to `pasg1lel0s...`. The log line
`repaired account numbers` in `new.log` lists the six accounts. `accounts.txt` compares each
account on mainnet and on the upgraded copy. Only the account number changes; the public key,
sequence, vesting periods and bank balance are identical.

On mainnet the six accounts get the next free account numbers at the upgrade height. The copy
created one account of its own first (its operator, number 11250), so the numbers there
(11251 to 11256) are one higher than mainnet would assign at the same height; accounts created
on mainnet before the upgrade move them further.

Impact on the holders of the five vesting accounts:

- Clients that read the account number from the chain before signing (wallets, `passage tx`
  without `--offline`) are not affected.
- Signatures made before the upgrade with account number 0 are invalid after it. This includes
  partially signed multisig transactions. Sign again after the upgrade, and take the account
  number from `passage q auth account <address>` when signing with `--offline`.

## Log messages

| Log | Message | Meaning |
|---|---|---|
| `old.log` | `error adding vote`, `cannot find validator N in valSet of size 1` at height 21712867 | The copied consensus WAL replays votes of the mainnet validators against the copy's single validator. Expected. |
| `old.log` | `CONSENSUS FAILURE ... UPGRADE "v4.1.0" NEEDED at height: 21712886` | The old binary halts at the upgrade height, as on mainnet. Expected. |
| `new.log` | `cannot get tx contracts from context` (125 times, during the smart queries) | wasmd v0.54 `checkDiscountEligibility` runs outside a transaction for a query and skips a gas discount. No state effect. |
| all | `Stopped accept routine`, `use of closed network connection` | The script stops the node. |

## Reproduce

Follow `scripts/rehearsal/README.md` with `UPG=v4.1.0`, the binaries listed above and a
passage-2 node at v4.0.3.
