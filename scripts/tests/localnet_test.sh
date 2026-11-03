#!/usr/bin/env bash
# Scenario test against the 4-validator localnet (make localnet-start).
# Runs every command inside the node containers, so only docker is needed on the host.
# Usage: scripts/tests/localnet_test.sh   -> prints PASS/FAIL lines, exit 1 on any FAIL
set -u
FAILS=0
ok()  { echo "PASS: $*"; }
bad() { echo "FAIL: $*"; FAILS=$((FAILS+1)); }
check() { local d=$1; shift; if "$@" >/dev/null 2>&1; then ok "$d"; else bad "$d"; fi; }

CHAIN=${CHAIN:-passage-localnet}
N0="docker exec passagenode0"
P="passage --home /root/.passage --keyring-backend test"
Q="passage --home /root/.passage"
TX="--chain-id $CHAIN --gas auto --gas-adjustment 1.5 --gas-prices 12.5upasg -y -o json"
CW20_URL=https://github.com/CosmWasm/cw-plus/releases/download/v1.1.2/cw20_base.wasm
CW20_SHA256=b29237070ebbc51c1130718cf1269d1b7fa4bdebef08edfe491c173e2eb31e4e

height() { docker exec "$1" sh -c 'curl -s localhost:26657/status | jq -r .result.sync_info.latest_block_height'; }
wait_height() { for _ in $(seq 1 120); do [ "$(height "$1")" -ge "$2" ] 2>/dev/null && return 0; sleep 1; done; return 1; }
txok() { # container, passage tx args...
  local c=$1; shift; local r h
  r=$(docker exec "$c" sh -c "$P tx $* $TX 2>/dev/null"); h=$(echo "$r" | jq -r .txhash 2>/dev/null)
  [ "$(echo "$r" | jq -r .code 2>/dev/null)" = "0" ] || { echo "  $(echo "$r" | jq -r .raw_log 2>/dev/null | head -c 200)"; return 1; }
  sleep 7; r=$(docker exec "$c" sh -c "$Q q tx $h -o json 2>/dev/null")
  [ "$(echo "$r" | jq -r .code)" = "0" ] || { echo "  $(echo "$r" | jq -r .raw_log | head -c 200)"; return 1; }; }

echo "== network"
H0=$(height passagenode0)
check "node0 producing blocks" wait_height passagenode0 $((H0 + 2))
check "4 bonded validators" sh -c "$N0 $Q q staking validators -o json | jq -e '[.validators[]|select(.status==\"BOND_STATUS_BONDED\" or .status==3)]|length==4'"
check "min commission param 5%" sh -c "$N0 $Q q staking params -o json | jq -e '(.params.min_commission_rate // .min_commission_rate)|startswith(\"0.05\")'"
check "bond denom upasg" sh -c "$N0 $Q q staking params -o json | jq -e '(.params.bond_denom // .bond_denom)==\"upasg\"'"

K0=$($N0 $P keys show passagenode0 -a); K1=$(docker exec passagenode1 $P keys show passagenode1 -a)

echo "== transactions"
check "bank send" txok passagenode0 bank send passagenode0 $K1 1000upasg
check "bank send signed amino-json (Ledger path)" txok passagenode0 bank send passagenode0 $K1 1000upasg --sign-mode amino-json
check "fee below the 12.5upasg floor rejected in CheckTx" sh -c "$N0 sh -c '$P tx bank send passagenode0 $K1 1upasg --chain-id $CHAIN --gas 200000 --fees 1upasg -y -o json' | jq -e '.code!=0'"

$N0 sh -c "curl -sSL -o /tmp/cw20.wasm $CW20_URL && echo '$CW20_SHA256  /tmp/cw20.wasm' | sha256sum -c -" >/dev/null || bad "download cw20"
check "wasm store" sh -c "docker exec passagenode0 sh -c \"$P tx wasm store /tmp/cw20.wasm --from passagenode0 --chain-id $CHAIN --gas 4000000 --gas-prices 12.5upasg -y -o json\" | jq -e '.code==0'"
sleep 7
CODE=$($N0 $Q q wasm list-code -o json | jq -r '.code_infos[-1].code_id')
INIT=$(jq -nc --arg a "$K0" '{name:"Local",symbol:"LCL",decimals:6,initial_balances:[{address:$a,amount:"1000000"}]}')
check "wasm instantiate (amino-json)" txok passagenode0 wasm instantiate $CODE "'$INIT'" --label local --no-admin --from passagenode0 --sign-mode amino-json
C=$($N0 $Q q wasm list-contract-by-code $CODE -o json | jq -r '.contracts[-1]')
check "wasm execute (amino-json, Ledger path)" txok passagenode0 wasm execute $C "'{\"transfer\":{\"recipient\":\"$K1\",\"amount\":\"10\"}}'" --from passagenode0 --sign-mode amino-json
check "cw20 balance on node3" sh -c "docker exec passagenode3 $Q q wasm contract-state smart $C '{\"balance\":{\"address\":\"$K1\"}}' -o json | jq -e '.data.balance==\"10\"'"

# authz MsgExec of a gzipped MsgStoreCode (needs the wasm GasRegisterDecorator).
check "authz grant store-code" txok passagenode0 wasm grant store-code $K1 "'*:*'" --from passagenode0
docker exec passagenode1 sh -c "curl -sSL -o /tmp/cw20.wasm $CW20_URL" >/dev/null
docker exec passagenode1 sh -c "$P tx wasm store /tmp/cw20.wasm --from $K0 --generate-only --chain-id $CHAIN > /tmp/store.json 2>/dev/null"
check "authz exec of gzipped MsgStoreCode" txok passagenode1 authz exec /tmp/store.json --from passagenode1 --gas 5000000 --gas-adjustment 1

echo "== consensus"
H=$(( $(height passagenode0) + 2 )); for n in 0 1 2 3; do wait_height passagenode$n $H; done
HASHES=$(for n in 0 1 2 3; do docker exec passagenode$n sh -c "curl -s 'localhost:26657/block?height=$H' | jq -r .result.block.header.app_hash"; done | sort -u | wc -l)
check "app_hash identical on 4 validators at height $H" test "$HASHES" = "1"
docker stop passagenode3 >/dev/null
H=$(height passagenode0); check "chain continues with 3/4 validators" wait_height passagenode0 $((H + 3))
docker start passagenode3 >/dev/null
check "stopped validator catches up" wait_height passagenode3 $(( $(height passagenode0) ))

echo "== node / API"
check "REST node config (/cosmos/base/node/v1beta1/config)" sh -c "$N0 sh -c 'curl -sf localhost:1317/cosmos/base/node/v1beta1/config' | jq -e '.minimum_gas_price|contains(\"upasg\")'"
check "REST swagger" $N0 sh -c 'curl -sf -o /dev/null localhost:1317/swagger/'
check "historical query (height-10)" $N0 sh -c "$Q q bank balances $K1 --height $(( $(height passagenode0) - 10 ))"
check "q block" $N0 sh -c "$Q q block --type=height 5"
check "q claim params" $N0 sh -c "$Q q claim params"
check "config command" $N0 sh -c "$Q config view client"

echo; [ $FAILS -eq 0 ] && echo "localnet test passed" || { echo "localnet test: $FAILS failure(s)"; exit 1; }
