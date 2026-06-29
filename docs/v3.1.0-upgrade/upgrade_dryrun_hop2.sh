#!/bin/bash
# HOP-2 dry-run: synthetic 0.47 -> gov v1 MsgSoftwareUpgrade "v050" -> halt ->
# swap 0.50 binary -> v050 handler (RunMigrations) -> chain continues ->
# REST/LCD QUERY BATTERY (the path Keplr/wallets use; the v3.0.0-outage catcher).
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
OLD=${OLD:-/tmp/passaged}; NEW=${NEW:-/tmp/passaged050}
H=/tmp/pasgupg2; CHAIN=pasg-upg2; KB="--keyring-backend test"
RPC=tcp://127.0.0.1:46657; NODE="--node $RPC"; LCD=http://127.0.0.1:41317
PORTS="--rpc.laddr tcp://127.0.0.1:46657 --p2p.laddr tcp://127.0.0.1:46656 --grpc.address 127.0.0.1:49090 --grpc-web.enable=false --rpc.pprof_laddr 127.0.0.1:46060 --api.enable=true --api.address tcp://127.0.0.1:41317"
TXF="$KB --home $H $NODE --chain-id $CHAIN --gas auto --gas-adjustment 1.4 --gas-prices 0stake -y"
[ -x "$OLD" ] || { echo "MISSING OLD $OLD"; exit 1; }; [ -x "$NEW" ] || { echo "MISSING NEW $NEW"; exit 1; }
pkill -9 -f "$OLD start" 2>/dev/null; pkill -9 -f "$NEW start" 2>/dev/null; sleep 1; rm -rf "$H"

echo "===== STAGE 1: init v0.47 chain ====="
"$OLD" init upgnode --chain-id "$CHAIN" --home "$H" >/dev/null 2>&1
"$OLD" version --long 2>&1 | grep cosmos_sdk_version | sed 's/^/OLD /'
python3 - "$H/config/genesis.json" <<'PYEOF'
import json,sys
p=sys.argv[1]; d=json.load(open(p)); g=d["app_state"]["gov"]
prm=g.setdefault("params",{})
prm["voting_period"]="20s"; prm["max_deposit_period"]="60s"
prm["min_deposit"]=[{"denom":"stake","amount":"1000000"}]
for k in ("voting_params","deposit_params"):
    g.pop(k,None)
json.dump(d,open(p,"w")); print("gov: voting=20s min_deposit=1000000stake")
PYEOF
"$OLD" keys add val0 $KB --home "$H" >/dev/null 2>&1
ADDR=$("$OLD" keys show val0 -a $KB --home "$H")
"$OLD" add-genesis-account "$ADDR" 1000000000000stake --home "$H" >/dev/null 2>&1
"$OLD" gentx val0 500000000000stake --chain-id "$CHAIN" $KB --home "$H" >/dev/null 2>&1
"$OLD" collect-gentxs --home "$H" >/dev/null 2>&1
"$OLD" validate-genesis "$H/config/genesis.json" 2>&1 | sed 's/^/  /'

echo "===== STAGE 2: start v0.47 node ====="
nohup "$OLD" start --home "$H" --minimum-gas-prices 0stake $PORTS > /tmp/upg2_old.log 2>&1 &
OLDPID=$!; echo "OLD_PID=$OLDPID"
for i in $(seq 1 25); do sleep 3
  CUR=$(sed -r "s/\x1B\[[0-9;]*[mK]//g" /tmp/upg2_old.log | grep -E "executed block" | grep -oE "height=[0-9]+" | grep -oE "[0-9]+" | tail -1)
  [ -n "$CUR" ] && [ "$CUR" -ge 3 ] && break
done
echo "  v0.47 height=$CUR"

echo "===== STAGE 3: schedule v050 upgrade (NO-QUERY tx path) ====="
UPGH=$((CUR + 35)); echo "  upgrade-height=$UPGH"
GOVAUTH=$(python3 - <<'PY'
import hashlib
def polymod(v):
    G=[0x3b6a57b2,0x26508e6d,0x1ea119fa,0x3d4233dd,0x2a1462b3];c=1
    for x in v:
        b=c>>25;c=((c&0x1ffffff)<<5)^x
        for i in range(5): c^=G[i] if (b>>i)&1 else 0
    return c
def hrpexp(h): return [ord(c)>>5 for c in h]+[0]+[ord(c)&31 for c in h]
def cs(h,d):
    v=hrpexp(h)+d;pm=polymod(v+[0]*6)^1
    return [(pm>>5*(5-i))&31 for i in range(6)]
def enc(h,d):
    C="qpzry9x8gf2tvdw0s3jn54khce6mua7l"
    return h+"1"+"".join(C[x] for x in d+cs(h,d))
def conv(b):
    acc=bits=0;r=[]
    for x in b:
        acc=(acc<<8)|x;bits+=8
        while bits>=5: bits-=5;r.append((acc>>bits)&31)
    if bits: r.append((acc<<(5-bits))&31)
    return r
print(enc("pasg",conv(hashlib.sha256(b"gov").digest()[:20])))
PY
)
echo "  gov authority=$GOVAUTH"
cat > /tmp/upg2_prop.json <<JSON
{ "messages": [ { "@type": "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade", "authority": "$GOVAUTH", "plan": { "name": "v050", "height": "$UPGH", "info": "" } } ], "metadata": "ipfs://none", "deposit": "10000000stake", "title": "v050", "summary": "0.47 to 0.50" }
JSON
GEN="$KB --home $H --chain-id $CHAIN --gas 600000 --gas-prices 0stake --account-number 0"
"$OLD" tx gov submit-proposal /tmp/upg2_prop.json --from val0 $GEN --sequence 1 --generate-only > /tmp/u1.json 2>/tmp/u1.err
"$OLD" tx sign /tmp/u1.json --from val0 $KB --home "$H" --chain-id "$CHAIN" --offline --account-number 0 --sequence 1 > /tmp/s1.json 2>/tmp/s1.err
"$OLD" tx broadcast /tmp/s1.json $NODE --broadcast-mode sync > /tmp/upg2_prop.log 2>&1
sleep 6
grep -iE "code|txhash|raw_log" /tmp/upg2_prop.log | head -3 | sed "s/^/   submit: /"
[ -s /tmp/u1.err ] && head -2 /tmp/u1.err | sed "s/^/   gen-err: /"
PID=1
echo "===== STAGE 4: vote yes (NO-QUERY) ====="
"$OLD" tx gov vote $PID yes --from val0 $GEN --sequence 2 --generate-only > /tmp/u2.json 2>/tmp/u2.err
"$OLD" tx sign /tmp/u2.json --from val0 $KB --home "$H" --chain-id "$CHAIN" --offline --account-number 0 --sequence 2 > /tmp/s2.json 2>/tmp/s2.err
"$OLD" tx broadcast /tmp/s2.json $NODE --broadcast-mode sync > /tmp/upg2_vote.log 2>&1
sleep 6
grep -iE "code|raw_log" /tmp/upg2_vote.log | head -1 | sed "s/^/   vote: /"

echo "===== STAGE 5: wait for halt at $UPGH ====="
for i in $(seq 1 60); do sleep 3
  grep -q 'UPGRADE "v050" NEEDED' /tmp/upg2_old.log 2>/dev/null && { echo "  HALT DETECTED"; break; }
  kill -0 "$OLDPID" 2>/dev/null || { echo "  old node exited"; break; }
done
echo "  last 0.47 height:"; sed -r "s/\x1B\[[0-9;]*[mK]//g" /tmp/upg2_old.log | grep -oE "executed block height=[0-9]+" | tail -1 | sed 's/^/   /'
kill -9 "$OLDPID" 2>/dev/null; sleep 2
echo "  upgrade-info.json:"; cat "$H/data/upgrade-info.json" 2>/dev/null | sed 's/^/   /'; echo

echo "===== STAGE 6: swap to v0.50, apply upgrade ====="
nohup "$NEW" start --home "$H" --minimum-gas-prices 0stake $PORTS > /tmp/upg2_new.log 2>&1 &
NEWPID=$!; echo "NEW_PID=$NEWPID"
for i in $(seq 1 50); do sleep 3
  NH=$(sed -r "s/\x1B\[[0-9;]*[mK]//g" /tmp/upg2_new.log | grep -E "executed block" | grep -oE "height=[0-9]+" | grep -oE "[0-9]+" | tail -1)
  [ -n "$NH" ] && [ "$NH" -ge $((UPGH+8)) ] && break
done
echo "  apply log:"; sed -r "s/\x1B\[[0-9;]*[mK]//g" /tmp/upg2_new.log | grep -iE "applying upgrade|migrat|v050|upgrade complete" | head -6 | sed 's/^/   /'
echo "  0.50 height (must be > $UPGH): ${NH:-NONE}"
echo "  load/panic errors?"; sed -r "s/\x1B\[[0-9;]*[mK]//g" /tmp/upg2_new.log | grep -iE "panic:|CONSENSUS FAILURE|wrong app hash|failed to load|version does not exist" | grep -v log_level | head -3 | sed 's/^/   /' || echo "   none"
[ -z "$NH" ] && { echo "  ABORT: 0.50 produced no blocks"; tail -5 /tmp/upg2_new.log | sed 's/^/   /'; kill -9 $NEWPID 2>/dev/null; exit 1; }

echo "===== QUERY BATTERY via REST/LCD (Keplr's path) ====="
sleep 4; PASS=1
chk(){ # name url [header]
  local n="$1" u="$2" hdr="$3" r
  if [ -n "$hdr" ]; then r=$(curl -s -H "$hdr" "$u"); else r=$(curl -s "$u"); fi
  if echo "$r" | grep -qiE "version does not exist|failed to load|not found|\"code\": *[1-9]"; then
    echo "  FAIL $n -> $(echo "$r" | head -c 160)"; PASS=0
  else echo "  OK   $n"; fi
}
RH=$((UPGH + 2)); OH=$((UPGH - 8))
chk "balance @latest"          "$LCD/cosmos/bank/v1beta1/balances/$ADDR"
chk "balance @recent($RH)"     "$LCD/cosmos/bank/v1beta1/balances/$ADDR" "x-cosmos-block-height: $RH"
chk "balance @HISTORICAL($OH)" "$LCD/cosmos/bank/v1beta1/balances/$ADDR" "x-cosmos-block-height: $OH"
chk "node_info"                "$LCD/cosmos/base/tendermint/v1beta1/node_info"
chk "consensus params"         "$LCD/cosmos/consensus/v1/params"
chk "total supply"             "$LCD/cosmos/bank/v1beta1/supply"
# expedited gov params must be populated post-migration
EV=$(curl -s "$LCD/cosmos/gov/v1/params/voting" | python3 -c "import json,sys;d=json.load(sys.stdin);p=d.get('params',d);print(p.get('expedited_voting_period') or '')" 2>/dev/null)
[ -n "$EV" ] && echo "  OK   expedited proposals (expedited_voting_period=$EV)" || { echo "  FAIL expedited gov params unset"; PASS=0; }
echo "===== VERDICT: $([ $PASS -eq 1 ] && echo 'ALL QUERIES PASS' || echo 'QUERY FAILURE -> BLOCK RELEASE') ====="
kill -9 "$NEWPID" 2>/dev/null
echo "DONE"
