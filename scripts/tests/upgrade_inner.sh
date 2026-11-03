#!/usr/bin/env bash
# Runs inside golang:1.22.11-bookworm (see upgrade_test.sh); the new binary is built with GOTOOLCHAIN=go1.25.14. /src = repo (read-only), /out = results.
set -u
OUT=/out
: > "$OUT/summary.txt"
exec > >(tee "$OUT/upgrade.log") 2>&1
step() { echo; echo "=== $(date +%T) $*"; }
ok()   { echo "PASS: $*"; echo "PASS: $*" >> "$OUT/summary.txt"; }
bad()  { echo "FAIL: $*"; echo "FAIL: $*" >> "$OUT/summary.txt"; }
check() { local d=$1; shift; if "$@"; then ok "$d"; else bad "$d"; fi; }

OLD_REF=${OLD_REF:-eea1f3e}; UPG=${UPG:-v4.1.0}
CW20_URL=https://github.com/CosmWasm/cw-plus/releases/download/v1.1.2/cw20_base.wasm
CW20_SHA256=b29237070ebbc51c1130718cf1269d1b7fa4bdebef08edfe491c173e2eb31e4e
GAS_PRICES=12.5upasg

apt-get update -qq >/dev/null && apt-get install -y -qq git jq curl procps >/dev/null 2>&1
git config --global --add safe.directory '*'

step "build old=$OLD_REF and new=working tree"
mkdir -p /b/new
git clone -q /src /b/old && (cd /b/old && git checkout -q "$OLD_REF")
tar -C /src --exclude=./build --exclude=./mytestnet --exclude=./.git -cf - . | tar -C /b/new -xf -
(cd /b/old && go build -tags "netgo ledger" -o /usr/local/bin/passage-old ./cmd/passage) || { bad "build old"; exit 1; }
(cd /b/new && GOTOOLCHAIN=go1.25.14 go build -tags "netgo ledger" -o /usr/local/bin/passage-new ./cmd/passage) || { bad "build new"; exit 1; }
OLD=passage-old; NEW=passage-new
curl -sSL -o /tmp/cw20.wasm "$CW20_URL" && echo "$CW20_SHA256  /tmp/cw20.wasm" | sha256sum -c - || { bad "download cw20"; exit 1; }

CHAIN=passage-upgrade; A=/n/a; B=/n/b; KR="--keyring-backend test"
mkdir -p /n
step "genesis (old binary)"
$OLD init a --chain-id $CHAIN --home $A >/dev/null 2>&1
$OLD init b --chain-id $CHAIN --home $B >/dev/null 2>&1
for k in val user1 user2; do $OLD keys add $k $KR --home $A >/dev/null 2>&1; done
VAL=$($OLD keys show val -a $KR --home $A); U1=$($OLD keys show user1 -a $KR --home $A); U2=$($OLD keys show user2 -a $KR --home $A)
G=$A/config/genesis.json
START=$(date -u -d '-1 hour' +%Y-%m-%dT%H:%M:%SZ)
jq --arg start "$START" '
  .app_state.staking.params.bond_denom="upasg" | .app_state.mint.params.mint_denom="upasg" | .app_state.crisis.constant_fee.denom="upasg"
  | .app_state.gov.params.min_deposit=[{"denom":"upasg","amount":"1000000"}] | .app_state.gov.params.expedited_min_deposit=[{"denom":"upasg","amount":"2000000"}]
  | .app_state.gov.params.voting_period="20s" | .app_state.gov.params.expedited_voting_period="10s"
  | .app_state.staking.params.min_commission_rate="0.050000000000000000"
  | .app_state.claim.params.airdrop_enabled=true | .app_state.claim.params.airdrop_start_time=$start
  | .app_state.claim.params.duration_until_decay="60s" | .app_state.claim.params.duration_of_decay="1s"' $G > /tmp/g && mv /tmp/g $G
$OLD add-genesis-account $VAL 100000000000000upasg --home $A $KR
$OLD add-genesis-account $U1 100000000000000upasg --home $A $KR
$OLD gentx val 50000000000000upasg --chain-id $CHAIN --commission-rate 0.05 --home $A $KR >/dev/null 2>&1
$OLD collect-gentxs --home $A >/dev/null 2>&1
# v4.0.3 bug: x/claim InitGenesis panics if a genesis account has account number 0.
jq '.app_state.auth.accounts |= (to_entries | map(.value.account_number = ((.key + 1)|tostring)) | map(.value))' $G > /tmp/g && mv /tmp/g $G
check "genesis valid (old)" $OLD validate-genesis --home $A
cp $G $B/config/genesis.json
NODEID=$($OLD comet show-node-id --home $A)
sed -i 's/^timeout_commit = .*/timeout_commit = "1s"/' $A/config/config.toml $B/config/config.toml
sed -i "s#^persistent_peers = .*#persistent_peers = \"$NODEID@127.0.0.1:26656\"#" $B/config/config.toml
BP="--rpc.laddr tcp://127.0.0.1:36657 --p2p.laddr tcp://0.0.0.0:36656 --grpc.address 127.0.0.1:19090 --api.address tcp://127.0.0.1:11317 --rpc.pprof_laddr 127.0.0.1:16060"
start() { nohup $1 start --home $2 --minimum-gas-prices $GAS_PRICES $4 > /n/$3.log 2>&1 & echo $!; }
height() { curl -s localhost:$1/status | jq -r .result.sync_info.latest_block_height 2>/dev/null; }
waith() { for _ in $(seq 1 180); do h=$(height $2); [ "${h:-0}" -ge "$1" ] 2>/dev/null && return 0; sleep 1; done; return 1; }
TX="$KR --home $A --chain-id $CHAIN --gas auto --gas-adjustment 1.5 --gas-prices $GAS_PRICES -y -o json"
txres() { local r h; r=$("$@" 2>/tmp/txerr); h=$(echo "$r" | jq -r .txhash 2>/dev/null)
  if [ "$(echo "$r" | jq -r .code 2>/dev/null)" != "0" ]; then echo "${r:-$(grep -v 'gas estimate' /tmp/txerr | tail -2)}"; return; fi
  sleep 3; $NEW q tx "$h" --home $A -o json 2>/dev/null; }
txok() { local r; r=$(txres "$@"); [ "$(echo "$r" | jq -r .code 2>/dev/null)" = "0" ] || { echo "$r" | jq -r .raw_log 2>/dev/null || echo "$r" | tail -3; return 1; }; }

PA=$(start $OLD $A a_old ""); PB=$(start $OLD $B b_old "$BP")
waith 3 26657 && ok "old chain produces blocks" || { bad "old chain start"; tail -30 /n/a_old.log; exit 1; }

step "state on the old binary"
check "bank send (old)" txok $OLD tx bank send user1 $U2 1234567upasg $TX
VALOPER=$($OLD keys show val --bech val -a $KR --home $A)
check "delegate (old)" txok $OLD tx staking delegate $VALOPER 1000000000upasg --from user1 $TX
check "authz grant (old)" txok $OLD tx authz grant $U2 send --spend-limit 1000upasg --from user1 $TX
check "feegrant (old)" txok $OLD tx feegrant grant user1 $U2 --spend-limit 5000000upasg $TX
jq -n --arg a "$U1" --arg b "$U2" '{members:[{address:$a,weight:"1",metadata:"m"},{address:$b,weight:"1",metadata:"m"}]}' > /tmp/members.json
check "group create (old)" txok $OLD tx group create-group user1 meta /tmp/members.json $TX
check "wasm store (old)" txok $OLD tx wasm store /tmp/cw20.wasm --from user1 $KR --home $A --chain-id $CHAIN --gas 4000000 --gas-prices $GAS_PRICES -y -o json
CODE=$($OLD q wasm list-code --home $A -o json | jq -r '.code_infos[-1].code_id')
INIT=$(jq -nc --arg a "$U1" '{name:"Regr",symbol:"RGR",decimals:6,initial_balances:[{address:$a,amount:"1000000"}]}')
check "wasm instantiate (old)" txok $OLD tx wasm instantiate $CODE "$INIT" --label regr --no-admin --from user1 $TX
C=$($OLD q wasm list-contract-by-code $CODE --home $A -o json | jq -r '.contracts[-1]')
check "wasm execute direct (old)" txok $OLD tx wasm execute $C "{\"transfer\":{\"recipient\":\"$U2\",\"amount\":\"100\"}}" --from user1 $TX
PRE_H=$(height 26657)
check "claim airdrop enabled before upgrade (old EndAirdrop runs every block)" test "$($OLD q claim params --home $A -o json | jq -r "(.params // .).airdrop_enabled")" = "true"

# MsgClaim is rejected by v4.0.3 with a signer error (missing cosmos.msg.v1.signer).
R=$($OLD tx claim claim ActionInitialClaim --from user1 $TX 2>&1)
echo "$R" | grep -q "cosmos.msg.v1.signer" && ok "old: MsgClaim fails with signer error (bug reproduced)" || bad "old: MsgClaim signer error not reproduced: $(echo $R | head -c 200)"

amino_exec() { # signer-bin broadcast-bin amount
  local j acc seq r c
  j=$($OLD q auth account $U1 --home $A -o json)
  acc=$(echo "$j" | jq -r '[..|.account_number?|select(.!=null)][0]'); seq=$(echo "$j" | jq -r '[..|.sequence?|select(.!=null)][0]')
  $1 tx wasm execute $C "{\"transfer\":{\"recipient\":\"$U2\",\"amount\":\"$3\"}}" --from $U1 --generate-only --gas 400000 --gas-prices $GAS_PRICES --chain-id $CHAIN --home $A $KR > /tmp/u.json 2>/dev/null
  $1 tx sign /tmp/u.json --from user1 --sign-mode amino-json --offline -a $acc -s ${seq:-0} --chain-id $CHAIN --home $A $KR > /tmp/s.json 2>/tmp/sign.err || { tail -2 /tmp/sign.err; return 2; }
  r=$($2 tx broadcast /tmp/s.json --home $A -o json 2>&1); c=$(echo "$r" | jq -r .code)
  echo "broadcast code=$c $(echo "$r" | jq -r .raw_log 2>/dev/null | head -c 160)"
  [ "$c" = "0" ] || return 1; sleep 3; [ "$($NEW q tx "$(echo "$r" | jq -r .txhash)" --home $A -o json 2>/dev/null | jq -r .code)" = "0" ]; }
amino_exec $NEW $OLD 7 > /tmp/amino_old.out; rc=$?; cat /tmp/amino_old.out
if [ $rc -ne 0 ] && grep -q "signature verification failed" /tmp/amino_old.out; then ok "old: Ledger-style amino-json wasm signature rejected (bug reproduced)"; else bad "old: amino-json wasm check inconclusive (rc=$rc)"; fi

# Legacy ParameterChangeProposal (x/claim params) cannot even be submitted on v4.0.3.
GOV=$($OLD q auth module-account gov --home $A -o json | jq -r '[..|.address?|select(.!=null)][0]')
legacy_prop() { jq -n --arg gov "$GOV" --arg v "$1" '{messages:[{"@type":"/cosmos.gov.v1.MsgExecLegacyContent",authority:$gov,
  content:{"@type":"/cosmos.params.v1beta1.ParameterChangeProposal",title:"claim decay",description:"claim decay",
  changes:[{subspace:"claim",key:"DurationOfDecay",value:("\""+$v+"\"")}]}}],deposit:"2000000upasg",title:"claim decay",summary:"claim decay",expedited:true}' > /tmp/legacy.json; }
legacy_prop 3000000000
R=$(txres $OLD tx gov submit-proposal /tmp/legacy.json --from val $TX)
[ "$(echo "$R" | jq -r .code 2>/dev/null)" != "0" ] && ok "old: legacy param-change proposal rejected (no legacy router, bug reproduced)" || bad "old: legacy proposal unexpectedly accepted"

step "software upgrade $UPG"
H=$(( $(height 26657) + 25 ))
jq -n --arg gov "$GOV" --arg n "$UPG" --argjson h $H '{messages:[{"@type":"/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",authority:$gov,plan:{name:$n,height:($h|tostring),info:""}}],deposit:"2000000upasg",title:"upgrade",summary:"upgrade",expedited:true}' > /tmp/prop.json
check "submit upgrade proposal" txok $OLD tx gov submit-proposal /tmp/prop.json --from val $TX
PID=$($OLD q gov proposals --home $A -o json | jq -r '.proposals[-1].id')
check "vote upgrade proposal" txok $OLD tx gov vote $PID yes --from val $TX --gas 400000
echo "upgrade height $H"; waith $((H-1)) 26657; sleep 10
check "old binary halts at the upgrade height" grep -q "UPGRADE \"$UPG\" NEEDED" /n/a_old.log
kill $PA $PB 2>/dev/null; sleep 3

step "new binary"
PA=$(start $NEW $A a_new ""); PB=$(start $NEW $B b_new "$BP")
waith $((H+10)) 26657 && ok "new binary produces blocks after the upgrade" || { bad "new binary after upgrade"; tail -40 /n/a_new.log; }
waith $((H+10)) 36657 && ok "full node follows after the upgrade" || { bad "full node after upgrade"; grep -iE "app.?hash|panic|CONSENSUS FAILURE" /n/b_new.log | tail -5; }
check "no consensus failure / panic in logs" bash -c "! grep -qiE 'CONSENSUS FAILURE|wrong Block.Header.AppHash|^panic' /n/a_new.log /n/b_new.log"
check "plan $UPG applied at $H" grep -q "applying upgrade \"$UPG\" at height: $H" /n/a_new.log

step "state preserved"
check "bank balance" test "$($NEW q bank balances $U2 --home $A -o json | jq -r '.balances[]|select(.denom=="upasg").amount')" = "1234567"
check "delegation" bash -c "$NEW q staking delegation $U1 $VALOPER --home $A -o json | jq -e '.delegation_response.balance.amount==\"1000000000\"' >/dev/null"
check "authz grant" bash -c "$NEW q authz grants $U1 $U2 --home $A -o json | jq -e '.grants|length==1' >/dev/null"
check "feegrant" bash -c "$NEW q feegrant grant $U1 $U2 --home $A -o json >/dev/null 2>&1"
check "group" bash -c "$NEW q group groups-by-admin $U1 --home $A -o json | jq -e '.groups|length==1' >/dev/null"
check "cw20 contract state" test "$($NEW q wasm contract-state smart $C "{\"balance\":{\"address\":\"$U2\"}}" --home $A -o json | jq -r .data.balance)" = "100"
check "historical query at pre-upgrade height $PRE_H" bash -c "$NEW q bank balances $U2 --height $PRE_H --home $A -o json | jq -e '.balances|length>=1' >/dev/null"

step "v4.1.0 fixes"
check "bank send (new)" txok $NEW tx bank send user1 $U2 1upasg $TX
check "wasm execute direct (new)" txok $NEW tx wasm execute $C "{\"transfer\":{\"recipient\":\"$U2\",\"amount\":\"5\"}}" --from user1 $TX
amino_exec $NEW $NEW 11 && ok "Ledger-style amino-json MsgExecuteContract accepted" || bad "amino-json MsgExecuteContract after upgrade"
check "amino-json bank send" txok $NEW tx bank send user1 $U2 3upasg $TX --sign-mode amino-json
check "expired airdrop disabled by EndAirdrop" test "$($NEW q claim params --home $A -o json | jq -r "(.params // .).airdrop_enabled")" = "false"
R=$(txres $NEW tx claim claim ActionInitialClaim --from user1 $TX)
echo "$R" | grep -q "cosmos.msg.v1.signer" && bad "new: MsgClaim still fails with signer error" || ok "new: MsgClaim signer resolved (module error only: $(echo "$R" | jq -r .raw_log 2>/dev/null | head -c 80))"
legacy_prop 2000000000
check "legacy param-change proposal submitted" txok $NEW tx gov submit-proposal /tmp/legacy.json --from val $TX
LPID=$($NEW q gov proposals --home $A -o json | jq -r '.proposals[-1].id')
check "legacy param-change vote" txok $NEW tx gov vote $LPID yes --from val $TX --gas 400000
sleep 15
check "legacy proposal passed" bash -c "$NEW q gov proposal $LPID --home $A -o json | jq -r '[..|.status?|select(.!=null)][0]' | grep -qE '^(PROPOSAL_STATUS_PASSED|3)$'"
check "x/claim param changed by governance" test "$($NEW q claim params --home $A -o json | jq -r "(.params // .).duration_of_decay")" = "2s"
H2=$(( $(height 26657) + 3 )); waith $H2 26657; waith $H2 36657
check "app_hash equal on both nodes at $H2" test "$(curl -s "localhost:26657/block?height=$H2" | jq -r .result.block.header.app_hash)" = "$(curl -s "localhost:36657/block?height=$H2" | jq -r .result.block.header.app_hash)"

step "node operations"
kill $PA $PB; sleep 4
check "node configured with 0upasg starts and enforces the 12.5upasg floor" bash -c "timeout 15 $NEW start --home $A --minimum-gas-prices 0upasg > /n/mingas.log 2>&1; grep -q \"using the floor instead\" /n/mingas.log"
check "export" bash -c "$NEW export --home $A --output-document $OUT/export.json 2>$OUT/export.err"
check "export contains the wasm contract" bash -c "jq -e '.app_state.wasm.contracts|length>=1' $OUT/export.json >/dev/null"
check "account numbers unique in export" bash -c "jq -e '[.app_state.auth.accounts[] | (.account_number // .base_account.account_number // .base_vesting_account.base_account.account_number)] | (length == (unique|length))' $OUT/export.json >/dev/null"
check "export --for-zero-height" bash -c "$NEW export --for-zero-height --home $A --output-document $OUT/export0.json 2>$OUT/export0.err"
mkdir -p /n/z/config && cp $OUT/export0.json /n/z/config/genesis.json
if $NEW validate-genesis /n/z/config/genesis.json --home /n/z > /n/validate0.log 2>&1; then ok "zero-height export is a valid genesis"; elif grep -q "connection identifier is not in the format" /n/validate0.log; then echo "KNOWN: validate-genesis rejects connection-localhost (ibc-go v8.0.0 bug, fixed in later v8 releases)" | tee -a $OUT/summary.txt; else bad "zero-height export is a valid genesis"; fi
check "rollback (full node)" bash -c "$NEW rollback --home $B > /n/rollback.log 2>&1"
PA=$(start $NEW $A a_new2 ""); PB=$(start $NEW $B b_new2 "$BP")
H3=$(( $(height 26657 || echo 0) + 5 ))
waith $H3 26657 && waith $H3 36657 && ok "both nodes continue after rollback" || bad "nodes after rollback"
kill $PA $PB 2>/dev/null

cp /n/*.log "$OUT/" 2>/dev/null
echo; echo "===== SUMMARY"; cat "$OUT/summary.txt"
