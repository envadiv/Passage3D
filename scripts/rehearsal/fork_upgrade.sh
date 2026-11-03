#!/usr/bin/env bash
# Upgrade rehearsal on a copy of a passage-2 node's state ("in-place testnet").
#
#   SRC_HOME  home of a STOPPED passage-2 node, or a filesystem snapshot of it (never written)
#   WORK      empty directory on the same filesystem (cp --reflink when supported)
#   OLD_BIN   version running on mainnet, with `passage in-place-testnet` (see README.md)
#   NEW_BIN   candidate binary
#   UPG       upgrade plan name of the candidate     DELAY  blocks until the upgrade (default 20)
#   PORT_OFFSET  added to every port, so a live node on the same host is not disturbed (default 1000)
#
# The copy gets a new validator key and node key, no peers or seeds, and listens on
# 127.0.0.1 only. The old binary takes the chain over with the new key, schedules $UPG
# and halts there; the new binary runs the upgrade. Results: $WORK/out/summary.txt
set -u
SRC_HOME=${SRC_HOME:?set SRC_HOME}; WORK=${WORK:?set WORK}; OLD_BIN=${OLD_BIN:?set OLD_BIN}; NEW_BIN=${NEW_BIN:?set NEW_BIN}
UPG=${UPG:?set UPG (upgrade plan name)}; DELAY=${DELAY:-20}; O=${PORT_OFFSET:-1000}; CHAIN=${CHAIN:-passage-fork}
H=$WORK/home; OUT=$WORK/out; RPC=tcp://127.0.0.1:$((26657 + O))
mkdir -p "$OUT"; : > "$OUT/summary.txt"
step()  { echo; echo "=== $(date +%T) $*"; }
ok()    { echo "PASS: $*" | tee -a "$OUT/summary.txt"; }
bad()   { echo "FAIL: $*" | tee -a "$OUT/summary.txt"; }
known() { echo "KNOWN: $*" | tee -a "$OUT/summary.txt"; }
die()   { bad "$*"; exit 1; }
q()     { "$1" q "${@:2}" --node $RPC -o json 2>/dev/null; }
height() { curl -s "127.0.0.1:$((26657 + O))/status" | jq -r .result.sync_info.latest_block_height 2>/dev/null; }
wait_height() { local h; for _ in $(seq 1 "$2"); do h=$(height); [ "${h:-0}" -ge "$1" ] 2>/dev/null && return 0; sleep 1; done; return 1; }

step "checks"
for b in "$OLD_BIN" "$NEW_BIN"; do [ -x "$b" ] || die "not executable: $b"; done
for t in jq curl; do command -v $t >/dev/null || die "missing tool: $t"; done
"$OLD_BIN" in-place-testnet --help >/dev/null 2>&1 || die "OLD_BIN has no in-place-testnet command"
[ -d "$SRC_HOME/data" ] && [ -d "$SRC_HOME/config" ] || die "SRC_HOME has no data/ or config/"
[ -d "$SRC_HOME/wasm" ] || die "SRC_HOME has no wasm/ (contract code)"
[ -e "$H" ] && die "$H exists; use an empty WORK"
# A node started without --home (e.g. `ExecStart=/usr/bin/passage start`) uses ~/.passage of
# the user it runs as, so its command line does not name SRC_HOME.
running_on_src() {
  local pid args home
  pgrep -af 'passage.* start' | grep -qF -- "$SRC_HOME" && return 0
  for pid in $(pgrep -f 'passage.* start'); do
    args=$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null) || continue
    case "$args" in *--home*) continue ;; esac
    home=$(getent passwd "$(stat -c %U "/proc/$pid")" | cut -d: -f6)
    [ -n "$home" ] && [ "$(readlink -f "$home/.passage")" = "$(readlink -f "$SRC_HOME")" ] && return 0
  done
  return 1
}
if running_on_src; then
  [ "${FORCE:-0}" = 1 ] || die "a node is running on SRC_HOME; stop it or use a snapshot (FORCE=1 to override)"
fi
need=$(du -sk "$SRC_HOME/data" "$SRC_HOME/wasm" | awk '{s += $1} END {print s}')
avail=$(df -Pk "$WORK" | awk 'NR == 2 {print $4}')
echo "state: $((need / 1024)) MiB, free in WORK: $((avail / 1048576)) GiB"
[ "$avail" -gt "$need" ] || echo "WARNING: not enough free space for a full copy; works only if cp --reflink applies"

step "copy (reflink when the filesystem supports it)"
mkdir -p "$H"
cp -a --reflink=auto "$SRC_HOME/config" "$SRC_HOME/data" "$SRC_HOME/wasm" "$H/" || die "copy failed"
# Never keep the original identity in the copy.
rm -f "$H/config/priv_validator_key.json" "$H/config/node_key.json"
"$OLD_BIN" init fork-keys --home "$WORK/keys" >/dev/null 2>&1
cp "$WORK/keys/config/priv_validator_key.json" "$WORK/keys/config/node_key.json" "$H/config/"
echo '{"height":"0","round":0,"step":0}' > "$H/data/priv_validator_state.json"
C=$H/config/config.toml; A=$H/config/app.toml
sed -i -E "s#^persistent_peers = .*#persistent_peers = \"\"#; s#^seeds = .*#seeds = \"\"#; s#^pex = .*#pex = false#;
  s#^unconditional_peer_ids = .*#unconditional_peer_ids = \"\"#; s#^private_peer_ids = .*#private_peer_ids = \"\"#;
  s#^external_address = .*#external_address = \"\"#; s#^pprof_laddr = .*#pprof_laddr = \"\"#;
  s#^laddr = \"tcp://[^:]*:26656\"#laddr = \"tcp://127.0.0.1:$((26656 + O))\"#;
  s#^laddr = \"tcp://[^:]*:26657\"#laddr = \"tcp://127.0.0.1:$((26657 + O))\"#;
  s#^proxy_app = .*#proxy_app = \"tcp://127.0.0.1:$((26658 + O))\"#;
  s#^prometheus = .*#prometheus = false#" "$C"
sed -i -E "s#^address = \"tcp://[^:]*:1317\"#address = \"tcp://127.0.0.1:$((1317 + O))\"#;
  s#^address = \"[^:]*:9090\"#address = \"127.0.0.1:$((9090 + O))\"#; s#^address = \"[^:]*:9091\"#address = \"127.0.0.1:$((9091 + O))\"#;
  s#^snapshot-interval = .*#snapshot-interval = 0#" "$A"
grep -q "127.0.0.1:$((26657 + O))" "$C" && ok "copy isolated (new keys, no peers, localhost ports +$O)" || die "could not rewrite config.toml"

step "take over the chain with the old binary"
KR="--keyring-backend test --home $H"
"$OLD_BIN" keys add operator $KR >/dev/null 2>&1; OP=$("$OLD_BIN" keys show operator -a $KR)
"$OLD_BIN" in-place-testnet "$CHAIN" "$OP" --home "$H" --skip-confirmation \
  --schedule-upgrade "$UPG" --schedule-upgrade-delay "$DELAY" > "$OUT/old.log" 2>&1 &
PID=$!
wait_height 1 600; H0=$(height); echo "copied state at height $H0; waiting for the first blocks of the fork"
wait_height $((H0 + 2)) 600 || die "fork does not produce blocks (see $OUT/old.log)"
UPH=$(q "$OLD_BIN" upgrade plan | jq -r '.plan.height // .height // empty')
[ -n "$UPH" ] && ok "old binary produces blocks (height $(height)); $UPG scheduled at $UPH" || die "fork did not start (see $OUT/old.log)"

snapshot() { # dir binary
  local d=$1 code c
  mkdir -p "$d"
  q "$2" wasm list-code --limit 100000 | jq -r '.code_infos[] | "\(.code_id) \(.data_hash)"' | sort -n > "$d/codes.txt"
  : > "$d/contracts.txt"
  for code in $(cut -d' ' -f1 "$d/codes.txt"); do
    for c in $(q "$2" wasm list-contract-by-code "$code" --limit 100000 | jq -r '.contracts[]'); do
      echo "$code $c $(q "$2" wasm contract-state all "$c" --limit 10000000 | jq -cS .models | sha256sum | cut -c1-64)" >> "$d/contracts.txt"
    done
  done
  q "$2" upgrade module-versions | jq -cS '.module_versions' > "$d/module-versions.json"
  q "$2" auth module-account claim | jq -r '[.. | .account_number? // empty][0]' > "$d/claim-number.txt"
  q "$2" auth module-account claim | jq -r '[.. | .address? // empty][0]' > "$d/claim-address.txt"
  q "$2" auth address-by-acc-num 0 | jq -r '.account_address // empty' > "$d/number0-owner.txt"
  echo "$(wc -l < "$d/codes.txt") codes, $(wc -l < "$d/contracts.txt") contracts"
}

for _ in $(seq 1 $((DELAY * 10 + 120))); do grep -q "UPGRADE \"$UPG\" NEEDED" "$OUT/old.log" && break; sleep 1; done
grep -q "UPGRADE \"$UPG\" NEEDED" "$OUT/old.log" && ok "old binary halts at the upgrade height" || die "old binary did not halt (see $OUT/old.log)"
step "snapshot before the upgrade"; snapshot "$OUT/before" "$OLD_BIN"
kill $PID; wait $PID 2>/dev/null; sleep 2

step "new binary"
"$NEW_BIN" start --home "$H" > "$OUT/new.log" 2>&1 &
PID=$!
wait_height $((UPH + 3)) 900 && ok "new binary produces blocks after the upgrade (height $(height))" || die "new binary does not produce blocks (see $OUT/new.log)"
grep -q "applying upgrade \"$UPG\"" "$OUT/new.log" && ok "upgrade handler $UPG applied" || bad "no 'applying upgrade' line in new.log"
step "snapshot after the upgrade"; snapshot "$OUT/after" "$NEW_BIN"

step "comparison"
diff "$OUT/before/codes.txt" "$OUT/after/codes.txt" > /dev/null && ok "wasm code hashes unchanged" || bad "wasm codes changed"
diff "$OUT/before/contracts.txt" "$OUT/after/contracts.txt" > "$OUT/contracts.diff" && ok "raw state of every contract unchanged" || bad "contract state changed (see contracts.diff)"
MV=$(jq -rn --slurpfile b "$OUT/before/module-versions.json" --slurpfile a "$OUT/after/module-versions.json" \
  '[($b[0] | map({(.name): (.version|tostring)}) | add) as $B | $a[0][] | select($B[.name] != (.version|tostring)) | "\(.name) \($B[.name] // "new")->\(.version)"] | join(", ")')
[ -z "$MV" ] && ok "module versions unchanged" || known "module versions changed by the upgrade: $MV"
CB=$(cat "$OUT/before/claim-number.txt"); CA=$(cat "$OUT/after/claim-number.txt"); CL=$(cat "$OUT/before/claim-address.txt")
OB=$(cat "$OUT/before/number0-owner.txt"); OA=$(cat "$OUT/after/number0-owner.txt")
echo "claim module account number: $CB -> $CA; owner of account number 0: $OB -> $OA"
if [ "$CB" = 0 ] && [ "$OB" != "$CL" ]; then
  [ "$CA" != 0 ] && [ "$OA" = "$OB" ] && ok "duplicate account number repaired (claim $CB -> $CA, number 0 kept by $OA)" || bad "account number repair: claim $CB -> $CA, number 0 $OB -> $OA"
else
  [ "$CA" = "$CB" ] && [ "$OA" = "$OB" ] && ok "account numbers unchanged (no duplicate to repair)" || bad "account numbers changed: claim $CB -> $CA, number 0 $OB -> $OA"
fi
V=$("$NEW_BIN" q wasm libwasmvm-version 2>/dev/null | tr -d '\r')
echo "libwasmvm: $V"; [ -n "$V" ] && ok "libwasmvm $V loaded" || bad "libwasmvm version not reported"

step "transactions and queries on the upgraded fork"
TX="--from operator $KR --chain-id $CHAIN --node $RPC --gas auto --gas-adjustment 1.5 --gas-prices 12.5upasg -y -o json"
# Many nodes run with indexer = "null" and the copy inherits it, so the transaction is not
# looked up by hash: a new account must receive the amount.
"$NEW_BIN" keys add probe $KR >/dev/null 2>&1; PROBE=$("$NEW_BIN" keys show probe -a $KR)
upasg() { q "$NEW_BIN" bank balances "$1" | jq -r '[.balances[]? | select(.denom == "upasg") | .amount][0] // "0"'; }
R=$("$NEW_BIN" tx bank send operator "$PROBE" 1upasg $TX 2>"$OUT/tx.err"); C=$(echo "$R" | jq -r .code 2>/dev/null)
for _ in $(seq 1 60); do [ "$(upasg "$PROBE")" = 1 ] && break; sleep 1; done
B=$(upasg "$PROBE")
[ "$C" = 0 ] && [ "$B" = 1 ] && ok "bank send (a new account received 1upasg)" \
  || bad "bank send: code ${C:-none}, received ${B:-?}upasg: $(tail -c 200 "$OUT/tx.err") $(echo "$R" | tail -c 200)"
: > "$OUT/smart-queries.txt"
for c in $(cut -d' ' -f2 "$OUT/after/contracts.txt"); do
  r=1
  for m in '{"contract_info":{}}' '{"config":{}}' '{"num_tokens":{}}' '{"ask_count":{}}' '{"params":{}}'; do
    q "$NEW_BIN" wasm contract-state smart "$c" "$m" >/dev/null && { r=0; break; }
  done
  echo "$c $r" >> "$OUT/smart-queries.txt"
done
n=$(wc -l < "$OUT/smart-queries.txt"); f=$(grep -vc ' 0$' "$OUT/smart-queries.txt")
[ "$f" = 0 ] && ok "smart queries answered by all $n contracts" || known "$f of $n contracts answered none of the probe queries (see smart-queries.txt)"

step "restart"
HB=$(height); kill $PID; wait $PID 2>/dev/null; sleep 2
"$NEW_BIN" start --home "$H" > "$OUT/new2.log" 2>&1 &
PID=$!
wait_height $((HB + 3)) 180 && ok "node restarts and continues (height $(height))" || bad "node did not continue after restart"
kill $PID 2>/dev/null; wait $PID 2>/dev/null

echo; echo "===== SUMMARY ($OUT/summary.txt)"
echo "PASS $(grep -c '^PASS' "$OUT/summary.txt")  FAIL $(grep -c '^FAIL' "$OUT/summary.txt")  KNOWN $(grep -c '^KNOWN' "$OUT/summary.txt")"
grep -E '^(FAIL|KNOWN)' "$OUT/summary.txt"
grep -q '^FAIL' "$OUT/summary.txt" && exit 1 || exit 0
