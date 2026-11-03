#!/usr/bin/env bash
# CosmWasm regression test for the Passage contracts (passage-contracts, legacy contracts
# excluded) and for the contract code stored on passage-2. Runs inside golang:1.22.11-bookworm (new binary: GOTOOLCHAIN=go1.25.14)
# (see contracts_test.sh):
#   /src        Passage3D repository (read-only)
#   /contracts  passage-contracts/artifacts (read-only)
#   /mainnet    optional: passage-2 code as code_<id>.wasm (otherwise downloaded over gRPC)
#   /out        results
#
# Old binary (OLD_REF, v4.0.3): store every code, run every scenario, leave in-flight state
# (open ask, open bid, running auction, unstaking NFT, pending friend request, open multisig
# proposal, minter with tokens left) -> software upgrade $UPG -> new binary: contract state
# identical, in-flight state completed, every scenario again on freshly stored code, old/new
# outcome comparison.
set -u
OUT=/out
: > "$OUT/summary.txt"
exec > >(tee "$OUT/contracts.log") 2>&1
step()  { echo; echo "=== $(date +%T) $*"; }
ok()    { echo "PASS: $*"; echo "PASS: $*" >> "$OUT/summary.txt"; }
bad()   { echo "FAIL: $*"; echo "FAIL: $*" >> "$OUT/summary.txt"; }
known() { echo "KNOWN: $*"; echo "KNOWN: $*" >> "$OUT/summary.txt"; }

OLD_REF=${OLD_REF:-eea1f3e}; UPG=${UPG:-v4.1.0}
MAINNET_RPC=${MAINNET_RPC:-https://passage-rpc.stakeandrelax.net:443}
MAINNET_GRPC=${MAINNET_GRPC:-passage-grpc.stakeandrelax.net:15690}
GAS_PRICES=12.5upasg
ARTIFACTS="auction_english follow friend marketplace_v2 minter minter_metadata_onchain nft_vault pg721 pg721_metadata_onchain royalty_group whitelist"
# passage-2 code ids, one per distinct checksum.
declare -A MAINNET_SHA=(
  [1]=184d596e2f4fe33d8ece99471b0f8d54483af6a506d8cbbce54cd046d95277b4   # pg721-metadata-onchain 0.2.0
  [2]=24539d4689c52bf0cedc6ffbf8f8a21bf4839e540e47a18dfb0136a585b0fd89   # minter-metadata-onchain 0.4.0
  [3]=d99776cdae47400d9285a456ed6d7d574bc7c5d6bcbbe4cc1a2900305dc84f63   # marketplace-v2
  [5]=91329fc2ffca493dc6c0e78d7031450b7a1230ad2625d3067bfea8cb2d1a1b6f   # pg721-metadata-onchain 0.3.0
  [27]=34127e578e8fecad20c726733b75dbbc2c013bc427f4c7448fbf832341ce188c  # marketplace-v2
  [28]=02057f47efaf01601a1979923d22b26c658384765ef6c382b660186760dfc88b  # whitelist 0.10.0
  [29]=d44e6aabf87d6cc0df6678103aaec4f2d2517e02429280fa269e1387089fc6fe  # cw4-group
  [30]=31e3dc29014cab8ea1b03f9799e32c94b29db5d74794ef700625c69fcb96e301  # cw3-flex-multisig
  [31]=2a09a54bf5edb25aad6e26ee21996838f3fbf86c48dd218f264ea5342805ea73  # marketplace-v2
  [32]=9438a8062fcfbca53c76b76f60491112cba2edbc1a0e9532c71bb4c30091957d  # pg721-metadata-onchain (= artifacts)
  [33]=c82aea935943ff62ac85aea6cb33bd8fae655fff6e4333bc924b71097c1ffb02  # pg721-metadata-onchain
)

apt-get update -qq >/dev/null && apt-get install -y -qq git jq curl procps >/dev/null 2>&1
git config --global --add safe.directory '*'

step "build old=$OLD_REF and new=working tree"
mkdir -p /b/new
tar -C /src --exclude=./build --exclude=./mytestnet --exclude=./.git -cf - . | tar -C /b/new -xf -
(cd /b/new && GOTOOLCHAIN=go1.25.14 go build -tags "netgo ledger" -o /usr/local/bin/passage-new ./cmd/passage) || { bad "build new"; exit 1; }
NEW=passage-new
if [ "${QUICK:-0}" = 1 ]; then OLD=$NEW; else   # QUICK=1: new binary only, no upgrade
  git clone -q /src /b/old && (cd /b/old && git checkout -q "$OLD_REF")
  (cd /b/old && go build -tags "netgo ledger" -o /usr/local/bin/passage-old ./cmd/passage) || { bad "build old"; exit 1; }
  OLD=passage-old
fi

step "contract code"
(cd /contracts && sha256sum -c checksums.txt --ignore-missing --quiet) && ok "artifacts match artifacts/checksums.txt" || bad "artifacts checksums"
mkdir -p /mn; MN=1
for id in "${!MAINNET_SHA[@]}"; do
  f=/mn/code_$id.wasm
  if [ -s /mainnet/code_$id.wasm ]; then cp /mainnet/code_$id.wasm $f
  else timeout 180 $NEW q wasm code $id $f --node $MAINNET_RPC --grpc-addr $MAINNET_GRPC --grpc-insecure >/dev/null 2>&1; fi
  [ "$(sha256sum $f 2>/dev/null | cut -c1-64)" = "${MAINNET_SHA[$id]}" ] || { MN=0; echo "passage-2 code $id: not available or checksum mismatch"; }
done
[ $MN = 1 ] && ok "passage-2 code available, checksums match" || known "passage-2 code not available (mount /mainnet or set MAINNET_GRPC); passage-2 scenarios skipped"

CHAIN=passage-contracts; A=/n/a; B=/n/b; KR="--keyring-backend test"
mkdir -p /n
step "genesis (old binary)"
$OLD init a --chain-id $CHAIN --home $A >/dev/null 2>&1
$OLD init b --chain-id $CHAIN --home $B >/dev/null 2>&1
for k in val u1 u2 u3 roy col; do $OLD keys add $k $KR --home $A >/dev/null 2>&1; done
addr() { $OLD keys show $1 -a $KR --home $A; }
VAL=$(addr val); U1=$(addr u1); U2=$(addr u2); U3=$(addr u3); ROY=$(addr roy); COL=$(addr col)
G=$A/config/genesis.json
jq '.app_state.staking.params.bond_denom="upasg" | .app_state.mint.params.mint_denom="upasg" | .app_state.crisis.constant_fee.denom="upasg"
  | .app_state.gov.params.min_deposit=[{"denom":"upasg","amount":"1000000"}] | .app_state.gov.params.expedited_min_deposit=[{"denom":"upasg","amount":"2000000"}]
  | .app_state.gov.params.voting_period="20s" | .app_state.gov.params.expedited_voting_period="10s"
  | .app_state.staking.params.min_commission_rate="0.050000000000000000"' $G > /tmp/g && mv /tmp/g $G
for a in $VAL $U1 $U2 $U3; do $OLD add-genesis-account $a 100000000000000upasg --home $A $KR; done
$OLD gentx val 50000000000000upasg --chain-id $CHAIN --commission-rate 0.05 --home $A $KR >/dev/null 2>&1
$OLD collect-gentxs --home $A >/dev/null 2>&1
# v4.0.3 bug: x/claim InitGenesis panics if a genesis account has account number 0.
jq '.app_state.auth.accounts |= (to_entries | map(.value.account_number = ((.key + 1)|tostring)) | map(.value))' $G > /tmp/g && mv /tmp/g $G
$OLD validate-genesis --home $A >/dev/null || { bad "genesis"; exit 1; }
cp $G $B/config/genesis.json
NODEID=$($OLD comet show-node-id --home $A)
sed -i 's/^timeout_commit = .*/timeout_commit = "1s"/' $A/config/config.toml $B/config/config.toml
sed -i "s#^persistent_peers = .*#persistent_peers = \"$NODEID@127.0.0.1:26656\"#" $B/config/config.toml
BP="--rpc.laddr tcp://127.0.0.1:36657 --p2p.laddr tcp://0.0.0.0:36656 --grpc.address 127.0.0.1:19090 --api.address tcp://127.0.0.1:11317 --rpc.pprof_laddr 127.0.0.1:16060"
start() { nohup $1 start --home $2 --minimum-gas-prices $GAS_PRICES $4 > /n/$3.log 2>&1 & echo $!; }
height() { curl -s localhost:$1/status | jq -r .result.sync_info.latest_block_height 2>/dev/null; }
waith() { for _ in $(seq 1 180); do h=$(height $2); [ "${h:-0}" -ge "$1" ] 2>/dev/null && return 0; sleep 1; done; return 1; }

# ---------------------------------------------------------------- helpers
TXF="$KR --home $A --chain-id $CHAIN --gas-prices $GAS_PRICES -y -o json"
GAS="--gas auto --gas-adjustment 1.5"
LAST=""
tx() { # bin tx-args... -> 0 when the tx is included with code 0; LAST = tx result or error text
  local bin=$1; shift; local r h
  r=$($bin tx "$@" $GAS $TXF 2>/tmp/txerr); h=$(echo "$r" | jq -r .txhash 2>/dev/null)
  if [ "$(echo "$r" | jq -r .code 2>/dev/null)" != "0" ]; then LAST="${r:-$(grep -v 'gas estimate' /tmp/txerr | tail -3)}"; return 1; fi
  for _ in $(seq 1 20); do sleep 1; LAST=$($NEW q tx "$h" --home $A -o json 2>/dev/null) && break; done
  [ "$(echo "$LAST" | jq -r .code 2>/dev/null)" = "0" ]; }
why() { local m; m=$(echo "$LAST" | jq -er .raw_log 2>/dev/null) || m=$LAST; echo "$m" | tr '\n' ' ' | head -c 300; }
# t / tno run a tx-producing command (tx, ex, approve) and record the outcome.
t()   { local d=$1; shift; if "$@"; then ok "$d"; else bad "$d: $(why)"; fi; }
tno() { local d=$1 e=$2; shift 2; if "$@"; then bad "$d: tx accepted"; elif why | grep -qi -- "$e"; then ok "$d"; else bad "$d: $(why)"; fi; }
ex()  { local bin=$1 from=$2 c=$3 m=$4; shift 4; tx $bin wasm execute "$c" "$m" --from $from "$@"; }
inst() { # bin code msg label -> ADDR (admin = val)
  ADDR=""; tx $1 wasm instantiate $2 "$3" --label "$4" --admin $VAL --from val || return 1
  ADDR=$(echo "$LAST" | jq -r '[.events[]|select(.type=="instantiate")|.attributes[]|select(.key=="_contract_address")|.value][0]')
  [ -n "$ADDR" ] && [ "$ADDR" != null ]; }
store() { # bin file -> STORED code id
  STORED=""; GAS="--gas 20000000" tx $1 wasm store "$2" --from val || return 1
  STORED=$(echo "$LAST" | jq -r '[.events[]|select(.type=="store_code")|.attributes[]|select(.key=="code_id")|.value][0]'); }
qs()  { $NEW q wasm contract-state smart "$1" "$2" --home $A -o json 2>/dev/null | jq -c .data; }
qc()  { local d=$1 r; r=$(qs "$2" "$3"); if echo "$r" | jq -e "$4" >/dev/null 2>&1; then ok "$d"; else bad "$d: $r"; fi; }
owner() { qs "$1" "{\"owner_of\":{\"token_id\":\"$2\"}}" | jq -r .owner; }
bal() { $NEW q bank balances "$1" --home $A -o json | jq -r '([.balances[]|select(.denom=="upasg").amount][0]) // "0"'; }
delta() { echo $(( $(bal $1) - $2 )); }
now() { date -d "$(curl -s localhost:26657/status | jq -r .result.sync_info.latest_block_time)" +%s; }
until_t() { for _ in $(seq 1 400); do [ "$(now)" -ge "$1" ] && return 0; sleep 1; done; return 1; }
ns() { echo "${1}000000000"; }
coin() { echo "{\"denom\":\"upasg\",\"amount\":\"$1\"}"; }
approve() { ex $BIN $1 $2 "{\"approve\":{\"spender\":\"$3\",\"token_id\":\"$4\",\"expires\":null}}"; }
raw() { $NEW q wasm contract-state raw "$1" "$2" --ascii --home $A -o json 2>/dev/null | jq -r '.data // empty' | base64 -d 2>/dev/null; }
# passage-2 minter / collection code (migration branch) is instantiated with
# migration_status.done=false and rejects every execute until its admin (minter code) or its
# minter (collection code) sends migration_done.
unlock() { # tag contract key
  [ "$(raw $2 migration_status | jq -r .done 2>/dev/null)" = false ] || return 0
  t "$1 migration_done" ex $BIN $3 $2 '{"migration_done":{}}'; }
declare -A CODE

# ---------------------------------------------------------------- scenarios
# All scenarios use the globals P (old|new) and BIN (signing binary). Test names never
# contain variable values so old/new outcomes can be compared.
store_all() {
  local n
  for n in $ARTIFACTS; do
    if store $BIN /contracts/$n.wasm; then CODE[$P:art:$n]=$STORED; ok "$P/art store $n"; else bad "$P/art store $n: $(why)"; fi
  done
  [ $MN = 1 ] || return 0
  for n in $(printf '%s\n' "${!MAINNET_SHA[@]}" | sort -n); do
    if store $BIN /mn/code_$n.wasm; then CODE[$P:mn:$n]=$STORED; ok "$P/mn store passage-2 code $n"; else bad "$P/mn store passage-2 code $n: $(why)"; fi
  done
}

# whitelist + minter-metadata-onchain (+ collection through submessage/reply) -> MINTER, COLL
nft_suite() { # set whitelist-code pg721-code minter-code [initial-collection-code]
  local tag="$P/$1" wl=$2 pg=$3 mm=$4 pg0=${5:-} T c meta id
  MINTER=""; COLL=""; T=$(now)
  inst $BIN $wl "$(jq -nc --arg u $U2 --arg s $(ns $((T+45))) --arg e $(ns $((T+60))) '{members:[$u],start_time:$s,end_time:$e,unit_price:{denom:"upasg",amount:"500000"},per_address_limit:1,member_limit:10}')" wl \
    && ok "$tag whitelist instantiate" || { bad "$tag whitelist instantiate: $(why)"; return; }
  local WL=$ADDR
  qc "$tag whitelist has_member" $WL "{\"has_member\":{\"member\":\"$U2\"}}" '.has_member==true'
  t  "$tag whitelist add_members" ex $BIN val $WL "{\"add_members\":{\"to_add\":[\"$U3\"]}}"
  qc "$tag whitelist config" $WL '{"config":{}}' '.num_members==2 and .is_active==false'
  tno "$tag whitelist add_members by non-admin rejected" "unauthorized" ex $BIN u1 $WL "{\"add_members\":{\"to_add\":[\"$U1\"]}}"
  c=$(jq -nc --arg v $VAL --arg r $ROY '{name:"Regression",symbol:"RGR",minter:$v,collection_info:{creator:$v,description:"regression collection",image:"ipfs://bafyregression/collection.png",external_link:"https://passage3d.com",royalty_info:{payment_address:$r,share:"0.05"}}}')
  if [ -n "$pg0" ]; then
    # passage-2 deployment path: collection instantiated on code 1 with an account as minter and
    # its migration closed, minter instantiated on the existing collection, collection migrated to
    # the newer code with the minter contract as minter.
    inst $BIN $pg0 "$c" collection && ok "$tag collection instantiate (passage-2 code 1)" || { bad "$tag collection instantiate: $(why)"; return; }
    COLL=$ADDR
    unlock "$tag collection" $COLL val
    inst $BIN $mm "$(jq -nc --argjson code $pg --arg s $(ns $((T+60))) --arg wl $WL --arg coll $COLL '{max_num_tokens:12,cw721_code_id:$code,start_time:$s,per_address_limit:3,unit_price:{denom:"upasg",amount:"1000000"},whitelist:$wl,cw721_address:$coll,cw721_instantiate_msg:null}')" minter \
      && ok "$tag minter instantiate (existing collection)" || { bad "$tag minter instantiate: $(why)"; return; }
    MINTER=$ADDR
    unlock "$tag minter" $MINTER val
    t "$tag migrate collection with the minter contract as minter" tx $BIN wasm migrate $COLL $pg "$(jq -nc --arg m $MINTER '{minter:$m}')" --from val
  else
    inst $BIN $mm "$(jq -nc --argjson c "$c" --argjson code $pg --arg s $(ns $((T+60))) --arg wl $WL '{max_num_tokens:12,cw721_code_id:$code,start_time:$s,per_address_limit:3,unit_price:{denom:"upasg",amount:"1000000"},whitelist:$wl,cw721_address:null,cw721_instantiate_msg:$c}')" minter \
      && ok "$tag minter instantiate (collection created through submessage reply)" || { bad "$tag minter instantiate: $(why)"; return; }
    MINTER=$ADDR; COLL=$(qs $MINTER '{"config":{}}' | jq -r .cw721_address)
  fi
  qc "$tag collection minter is the minter contract" $COLL '{"minter":{}}' ".minter==\"$MINTER\""
  qc "$tag collection royalty info" $COLL '{"collection_info":{}}' ".royalty_info.payment_address==\"$ROY\" and (.royalty_info.share|tonumber)==0.05"
  meta=$(jq -nc '[range(1;13)|{token_id:.,metadata:{name:("Token "+(.|tostring)),description:"regression",image:("ipfs://bafyregression/"+(.|tostring)+".png"),attributes:[{trait_type:"rank",value:(.|tostring)}]}}]')
  t "$tag minter upsert_token_metadatas" ex $BIN val $MINTER "{\"upsert_token_metadatas\":{\"token_metadatas\":$meta}}"
  for id in 5 6 7 8 9 10 11; do tx $BIN wasm execute $MINTER "{\"mint_for\":{\"token_id\":$id,\"recipient\":\"$U1\"}}" --from val || bad "$tag admin mint_for: $(why)"; done
  qc "$tag admin mint_for (free for admin)" $COLL "{\"tokens\":{\"owner\":\"$U1\",\"limit\":30}}" '.tokens|length==7'
  qc "$tag on-chain metadata" $COLL '{"nft_info":{"token_id":"5"}}' '.extension.name=="Token 5" and .extension.attributes[0].value=="5"'
  tno "$tag mint before start rejected" "not started" ex $BIN u1 $MINTER '{"mint":{}}' --amount 1000000upasg
  until_t $((T+46))
  qc "$tag whitelist active" $WL '{"is_active":{}}' '.is_active==true'
  tno "$tag whitelist phase, non-member rejected" "not on whitelist" ex $BIN u1 $MINTER '{"mint":{}}' --amount 500000upasg
  tno "$tag whitelist phase, public price rejected" "IncorrectPaymentAmount" ex $BIN u2 $MINTER '{"mint":{}}' --amount 1000000upasg
  t   "$tag whitelist mint at whitelist price" ex $BIN u2 $MINTER '{"mint":{}}' --amount 500000upasg
  tno "$tag whitelist per-address limit" "limit per address" ex $BIN u2 $MINTER '{"mint":{}}' --amount 500000upasg
  until_t $((T+61))
  t  "$tag public mint" ex $BIN u1 $MINTER '{"mint":{}}' --amount 1000000upasg
  qc "$tag num_minted" $MINTER '{"num_minted":{}}' '.num_minted==9'
  qc "$tag collection num_tokens" $COLL '{"num_tokens":{}}' '.count==9'
  local w0; w0=$(bal $U3)
  t "$tag minter withdraw" ex $BIN val $MINTER "{\"withdraw\":{\"recipient\":\"$U3\"}}"
  [ "$(delta $U3 $w0)" = 1500000 ] && ok "$tag minter withdraw amount" || bad "$tag minter withdraw amount: $(delta $U3 $w0)"
  tno "$tag withdraw by non-admin rejected" "unauthorized" ex $BIN u1 $MINTER "{\"withdraw\":{\"recipient\":\"$U1\"}}"
}

# marketplace-v2 on COLL: ask/buy with fee and royalty, offer/accept, collection offer -> MK
market_suite() { # set market-code collection token(owned by u1)
  local tag="$P/$1" mk=$2 coll=$3 tok=$4 s0 r0 c0
  MK=""
  inst $BIN $mk "$(jq -nc --arg c $coll --arg col $COL --arg v $VAL '{cw721_address:$c,denom:"upasg",collector_address:$col,trading_fee_bps:200,operators:[$v],min_price:"100"}')" market \
    && ok "$tag marketplace instantiate" || { bad "$tag marketplace instantiate: $(why)"; return; }
  MK=$ADDR
  t  "$tag approve marketplace" approve u1 $coll $MK $tok
  tno "$tag set_ask below min_price rejected" "price" ex $BIN u1 $MK "{\"set_ask\":{\"token_id\":\"$tok\",\"price\":$(coin 10),\"funds_recipient\":null}}"
  t  "$tag set_ask (NFT escrowed)" ex $BIN u1 $MK "{\"set_ask\":{\"token_id\":\"$tok\",\"price\":$(coin 2000000),\"funds_recipient\":null}}"
  [ "$(owner $coll $tok)" = "$MK" ] && ok "$tag NFT held by marketplace" || bad "$tag NFT held by marketplace"
  qc "$tag ask query" $MK "{\"ask\":{\"token_id\":\"$tok\"}}" ".ask.seller==\"$U1\""
  s0=$(bal $U1); r0=$(bal $ROY); c0=$(bal $COL)
  t  "$tag buy (set_bid matching the ask)" ex $BIN u2 $MK "{\"set_bid\":{\"token_id\":\"$tok\",\"price\":$(coin 2000000)}}" --amount 2000000upasg
  [ "$(owner $coll $tok)" = "$U2" ] && ok "$tag buyer owns NFT" || bad "$tag buyer owns NFT"
  if [ "$(delta $COL $c0)" = 40000 ] && [ "$(delta $ROY $r0)" = 100000 ] && [ "$(delta $U1 $s0)" = 1860000 ]; then ok "$tag payouts (2% fee, 5% royalty, rest to seller)"
  else bad "$tag payouts: fee $(delta $COL $c0) royalty $(delta $ROY $r0) seller $(delta $U1 $s0)"; fi
  t  "$tag offer on unlisted NFT (escrowed)" ex $BIN u1 $MK "{\"set_bid\":{\"token_id\":\"$tok\",\"price\":$(coin 3000000)}}" --amount 3000000upasg
  qc "$tag bid query" $MK "{\"bid\":{\"token_id\":\"$tok\",\"bidder\":\"$U1\"}}" '.bid.price.amount=="3000000"'
  t  "$tag owner approves marketplace" approve u2 $coll $MK $tok
  t  "$tag accept_bid" ex $BIN u2 $MK "{\"accept_bid\":{\"token_id\":\"$tok\",\"bidder\":\"$U1\"}}"
  [ "$(owner $coll $tok)" = "$U1" ] && ok "$tag bidder owns NFT" || bad "$tag bidder owns NFT"
  t  "$tag set_collection_bid" ex $BIN u2 $MK "{\"set_collection_bid\":{\"units\":1,\"price\":$(coin 1500000)}}" --amount 1500000upasg
  t  "$tag approve for collection bid" approve u1 $coll $MK $tok
  t  "$tag accept_collection_bid" ex $BIN u1 $MK "{\"accept_collection_bid\":{\"token_id\":\"$tok\",\"bidder\":\"$U2\"}}"
  [ "$(owner $coll $tok)" = "$U2" ] && ok "$tag collection bidder owns NFT" || bad "$tag collection bidder owns NFT"
}

# auction-english: reserve auction outbid + finalize, auction closed without bids -> AU
auction_suite() { # set auction-code collection token(owned by u1)
  local tag="$P/$1" code=$2 coll=$3 tok=$4 T b0 s0 end
  AU=""
  inst $BIN $code "$(jq -nc --arg c $coll --arg col $COL --arg v $VAL '{cw721_address:$c,denom:"upasg",collector_address:$col,trading_fee_bps:200,operators:[$v],min_price:"100",min_bid_increment:"100",min_duration:10,max_duration:3600,closed_duration:600,buffer_duration:5}')" auction \
    && ok "$tag auction instantiate" || { bad "$tag auction instantiate: $(why)"; return; }
  AU=$ADDR
  t  "$tag approve auction" approve u1 $coll $AU $tok
  T=$(now)
  t  "$tag set_auction (NFT escrowed)" ex $BIN u1 $AU "{\"set_auction\":{\"token_id\":\"$tok\",\"start_time\":\"$(ns $((T+10)))\",\"end_time\":\"$(ns $((T+30)))\",\"starting_price\":$(coin 1000),\"reserve_price\":$(coin 5000),\"funds_recipient\":null}}"
  tno "$tag bid before start rejected" "status" ex $BIN u2 $AU "{\"set_auction_bid\":{\"token_id\":\"$tok\",\"price\":$(coin 1000)}}" --amount 1000upasg
  until_t $((T+11))
  t  "$tag bid" ex $BIN u2 $AU "{\"set_auction_bid\":{\"token_id\":\"$tok\",\"price\":$(coin 1000)}}" --amount 1000upasg
  b0=$(bal $U2)
  t  "$tag outbid" ex $BIN u3 $AU "{\"set_auction_bid\":{\"token_id\":\"$tok\",\"price\":$(coin 6000)}}" --amount 6000upasg
  [ "$(delta $U2 $b0)" = 1000 ] && ok "$tag outbid bidder refunded" || bad "$tag outbid bidder refunded: $(delta $U2 $b0)"
  qc "$tag highest bid and reserve met" $AU "{\"auction\":{\"token_id\":\"$tok\"}}" ".auction.highest_bid.bidder==\"$U3\" and .is_reserve_price_met==true"
  tno "$tag finalize while open rejected" "status" ex $BIN val $AU "{\"finalize_auction\":{\"token_id\":\"$tok\"}}"
  end=$(qs $AU "{\"auction\":{\"token_id\":\"$tok\"}}" | jq -r '.auction.end_time|tonumber/1000000000|floor')
  until_t $((end+1)); s0=$(bal $U1)
  t  "$tag finalize_auction" ex $BIN val $AU "{\"finalize_auction\":{\"token_id\":\"$tok\"}}"
  [ "$(owner $coll $tok)" = "$U3" ] && ok "$tag winner owns NFT" || bad "$tag winner owns NFT"
  [ "$(delta $U1 $s0)" = 5580 ] && ok "$tag seller payout (fee and royalty deducted)" || bad "$tag seller payout: $(delta $U1 $s0)"
  t  "$tag approve auction (second)" approve u3 $coll $AU $tok
  T=$(now)
  t  "$tag set_auction without reserve" ex $BIN u3 $AU "{\"set_auction\":{\"token_id\":\"$tok\",\"start_time\":\"$(ns $((T+10)))\",\"end_time\":\"$(ns $((T+60)))\",\"starting_price\":$(coin 1000),\"reserve_price\":null,\"funds_recipient\":null}}"
  tno "$tag close by non-seller rejected" "unauthorized" ex $BIN u1 $AU "{\"close_auction\":{\"token_id\":\"$tok\",\"accept_highest_bid\":false}}"
  t  "$tag close_auction (no bids, NFT returned)" ex $BIN u3 $AU "{\"close_auction\":{\"token_id\":\"$tok\",\"accept_highest_bid\":false}}"
  [ "$(owner $coll $tok)" = "$U3" ] && ok "$tag NFT returned to seller" || bad "$tag NFT returned to seller"
}

# nft-vault: stake with a failing hook (reply_on_error), unstake, withdraw after the period -> VA
vault_suite() { # set vault-code collection token(owned by u1) hook-contract
  local tag="$P/$1" code=$2 coll=$3 tok=$4 hook=$5
  VA=""
  inst $BIN $code "$(jq -nc --arg c $coll '{cw721_address:$c,label:"regression vault",unstake_period:8}')" vault \
    && ok "$tag vault instantiate" || { bad "$tag vault instantiate: $(why)"; return; }
  VA=$ADDR
  t  "$tag add_stake_hook" ex $BIN val $VA "{\"add_stake_hook\":{\"hook\":\"$hook\"}}"
  tno "$tag add_stake_hook by non-operator rejected" "unauthorized" ex $BIN u1 $VA "{\"add_stake_hook\":{\"hook\":\"$U1\"}}"
  t  "$tag approve vault" approve u1 $coll $VA $tok
  t  "$tag stake" ex $BIN u1 $VA "{\"stake\":{\"token_id\":\"$tok\"}}"
  echo "$LAST" | jq -e '[.events[].attributes[]|select(.key=="action" and .value=="stake-hook-failed")]|length>=1' >/dev/null \
    && ok "$tag failing hook recovered by reply_on_error (stake kept)" || bad "$tag failing hook event missing"
  [ "$(owner $coll $tok)" = "$VA" ] && ok "$tag NFT held by vault" || bad "$tag NFT held by vault"
  qc "$tag vault_token owner" $VA "{\"vault_token\":{\"token_id\":\"$tok\"}}" ".vault_token.owner==\"$U1\""
  t  "$tag unstake" ex $BIN u1 $VA "{\"unstake\":{\"token_id\":\"$tok\"}}"
  tno "$tag withdraw during unstake period rejected" "InvalidStatus" ex $BIN u1 $VA "{\"withdraw\":{\"token_id\":\"$tok\"}}"
  sleep 10
  t  "$tag withdraw" ex $BIN u1 $VA "{\"withdraw\":{\"token_id\":\"$tok\"}}"
  [ "$(owner $coll $tok)" = "$U1" ] && ok "$tag NFT back to owner" || bad "$tag NFT back to owner"
}

royalty_suite() { # set royalty-group-code
  local tag="$P/$1" b1 b2
  inst $BIN $2 "$(jq -nc --arg v $VAL --arg a $U1 --arg b $U2 '{admin:$v,members:[{addr:$a,weight:1},{addr:$b,weight:3}]}')" royalty \
    && ok "$tag royalty-group instantiate" || { bad "$tag royalty-group instantiate: $(why)"; return; }
  local RG=$ADDR
  qc "$tag total_weight" $RG '{"total_weight":{}}' '.weight==4'
  t  "$tag fund group" tx $BIN bank send val $RG 4000upasg
  b1=$(bal $U1); b2=$(bal $U2)
  t  "$tag distribute" ex $BIN val $RG "{\"distribute\":{\"funds\":$(coin 4000)}}"
  [ "$(delta $U1 $b1)" = 1000 ] && [ "$(delta $U2 $b2)" = 3000 ] && ok "$tag distribution by weight" || bad "$tag distribution: $(delta $U1 $b1) $(delta $U2 $b2)"
  t  "$tag update_members" ex $BIN val $RG "{\"update_members\":{\"remove\":[\"$U1\"],\"add\":[{\"addr\":\"$U3\",\"weight\":4}]}}"
  qc "$tag total_weight after update" $RG '{"total_weight":{}}' '.weight==7'
  tno "$tag update_members by non-admin rejected" "not admin" ex $BIN u1 $RG "{\"update_members\":{\"remove\":[],\"add\":[{\"addr\":\"$U1\",\"weight\":9}]}}"
}

# follow + friend -> FO, FR
social_suite() { # set follow-code friend-code
  local tag="$P/$1"
  FO=""; FR=""
  inst $BIN $2 '{}' follow && ok "$tag follow instantiate" || { bad "$tag follow instantiate: $(why)"; return; }
  FO=$ADDR
  t  "$tag add_follow" ex $BIN u1 $FO "{\"add_follow\":{\"target\":\"$U2\"}}"
  qc "$tag is_follow" $FO "{\"is_follow\":{\"origin\":\"$U1\",\"target\":\"$U2\"}}" '.is_follow==true'
  # follow/src/state.rs: the followers index is keyed by origin and the following index by target
  # (swapped), so followers{target} returns the follows made BY target.
  local fr; fr=$(qs $FO "{\"followers\":{\"target\":\"$U2\",\"query_options\":{}}}")
  if echo "$fr" | jq -e '.follows|length==1' >/dev/null 2>&1; then ok "$tag followers"
  elif qs $FO "{\"followers\":{\"target\":\"$U1\",\"query_options\":{}}}" | jq -e '.follows|length==1' >/dev/null 2>&1; then
    known "$tag followers: returns the follows made by target (followers/following indexes swapped in follow state.rs)"
  else bad "$tag followers: $fr"; fi
  t  "$tag remove_follow" ex $BIN u1 $FO "{\"remove_follow\":{\"target\":\"$U2\"}}"
  qc "$tag is_follow after remove" $FO "{\"is_follow\":{\"origin\":\"$U1\",\"target\":\"$U2\"}}" '.is_follow==false'
  inst $BIN $3 '{}' friend && ok "$tag friend instantiate" || { bad "$tag friend instantiate: $(why)"; return; }
  FR=$ADDR
  t  "$tag add_friend (request)" ex $BIN u1 $FR "{\"add_friend\":{\"target\":\"$U2\"}}"
  qc "$tag received_friend_requests" $FR "{\"received_friend_requests\":{\"user\":\"$U2\",\"query_options\":{}}}" '.friends|length==1'
  tno "$tag accept by wrong user rejected" "" ex $BIN u3 $FR "{\"accept_friend_request\":{\"origin\":\"$U1\"}}"
  t  "$tag accept_friend_request" ex $BIN u2 $FR "{\"accept_friend_request\":{\"origin\":\"$U1\"}}"
  qc "$tag is_friend" $FR "{\"is_friend\":{\"origin\":\"$U1\",\"target\":\"$U2\"}}" '.is_friend==true'
  qc "$tag friends" $FR "{\"friends\":{\"user\":\"$U1\",\"query_options\":{}}}" '.friends|length==1'
  t  "$tag remove_friend" ex $BIN u1 $FR "{\"remove_friend\":{\"friend_addr\":\"$U2\"}}"
  qc "$tag is_friend after remove" $FR "{\"is_friend\":{\"origin\":\"$U1\",\"target\":\"$U2\"}}" '.is_friend==false'
}

# minter (token_uri based) + pg721
classic_suite() { # set minter-code pg721-code
  local tag="$P/$1" T c tok
  T=$(now)
  c=$(jq -nc --arg v $VAL --arg r $ROY '{name:"Classic",symbol:"CLS",minter:$v,collection_info:{creator:$v,description:"classic collection",image:"ipfs://bafyclassic/collection.png",external_link:null,royalty_info:{payment_address:$r,share:"0.1"}}}')
  inst $BIN $2 "$(jq -nc --argjson c "$c" --argjson code $3 --arg s $(ns $((T+12))) '{base_token_uri:"ipfs://bafyclassic",num_tokens:3,cw721_code_id:$code,cw721_instantiate_msg:$c,start_time:$s,per_address_limit:2,unit_price:{denom:"upasg",amount:"1000000"},whitelist:null}')" classic \
    && ok "$tag minter instantiate (pg721 through submessage reply)" || { bad "$tag minter instantiate: $(why)"; return; }
  local CM=$ADDR CC
  CC=$(qs $CM '{"config":{}}' | jq -r .cw721_address)
  until_t $((T+13))
  tno "$tag mint with wrong payment rejected" "IncorrectPaymentAmount" ex $BIN u1 $CM '{"mint":{}}' --amount 1upasg
  t  "$tag mint" ex $BIN u1 $CM '{"mint":{}}' --amount 1000000upasg
  tok=$(qs $CC "{\"tokens\":{\"owner\":\"$U1\"}}" | jq -r '.tokens[0]')
  qc "$tag token_uri from base uri" $CC "{\"nft_info\":{\"token_id\":\"$tok\"}}" '.token_uri|startswith("ipfs://bafyclassic/")'
  qc "$tag mintable_num_tokens" $CM '{"mintable_num_tokens":{}}' '.count==2'
  t  "$tag transfer_nft" ex $BIN u1 $CC "{\"transfer_nft\":{\"recipient\":\"$U2\",\"token_id\":\"$tok\"}}"
  [ "$(owner $CC $tok)" = "$U2" ] && ok "$tag new owner" || bad "$tag new owner"
  t  "$tag withdraw" ex $BIN val $CM '{"withdraw":{}}'
  [ "$(bal $CM)" = 0 ] && ok "$tag minter balance withdrawn" || bad "$tag minter balance: $(bal $CM)"
}

# pg721-metadata-onchain used directly (passage-2 collection code variants)
pg_suite() { # set pg721-code
  local tag="$P/$1"
  inst $BIN $2 "$(jq -nc --arg v $VAL '{name:"Direct",symbol:"DIR",minter:$v,collection_info:{creator:$v,description:"direct",image:"ipfs://bafydirect/c.png",external_link:null,royalty_info:null}}')" pg \
    || { if why | grep -q 'dlmalloc'; then
           # code 33: built with a newer rustc (dlmalloc 0.2.6) against cosmwasm-std 1.1.9; the
           # allocator assertion aborts the contract on its first call (0 instances on passage-2).
           known "$tag collection instantiate: contract aborts in its allocator (dlmalloc 0.2.6 with cosmwasm-std 1.1.9)"
         else bad "$tag collection instantiate: $(why)"; fi; return; }
  ok "$tag collection instantiate"
  local PG=$ADDR
  unlock "$tag collection" $PG val
  t  "$tag mint with metadata" ex $BIN val $PG "{\"mint\":{\"token_id\":\"1\",\"owner\":\"$U1\",\"token_uri\":null,\"extension\":{\"name\":\"Direct 1\",\"image\":\"ipfs://bafydirect/1.png\"}}}"
  tno "$tag mint by non-minter rejected" "unauthorized" ex $BIN u1 $PG "{\"mint\":{\"token_id\":\"2\",\"owner\":\"$U1\",\"token_uri\":null,\"extension\":null}}"
  qc "$tag nft_info" $PG '{"nft_info":{"token_id":"1"}}' '.extension.name=="Direct 1"'
  t  "$tag transfer_nft" ex $BIN u1 $PG "{\"transfer_nft\":{\"recipient\":\"$U2\",\"token_id\":\"1\"}}"
  [ "$(owner $PG 1)" = "$U2" ] && ok "$tag new owner" || bad "$tag new owner"
  t  "$tag burn" ex $BIN u2 $PG '{"burn":{"token_id":"1"}}'
  qc "$tag num_tokens after burn" $PG '{"num_tokens":{}}' '.count==0'
}

# passage-2 minter code creating its collection through the submessage: the collection starts with
# migration_status.done=false and accepts migration_done only from its minter, the minter
# contract, which has no message that sends it.
submsg_lock_check() { # set minter-code pg721-code
  local tag="$P/$1" c M C
  c=$(jq -nc --arg v $VAL '{name:"Locked",symbol:"LCK",minter:$v,collection_info:{creator:$v,description:"submessage collection",image:"ipfs://bafylocked/c.png",external_link:null,royalty_info:null}}')
  inst $BIN $2 "$(jq -nc --argjson c "$c" --argjson code $3 --arg s $(ns $(( $(now) + 10 ))) '{max_num_tokens:2,cw721_code_id:$code,start_time:$s,per_address_limit:2,unit_price:{denom:"upasg",amount:"1000000"},whitelist:null,cw721_address:null,cw721_instantiate_msg:$c}')" locked \
    && ok "$tag submessage minter instantiate" || { bad "$tag submessage minter instantiate: $(why)"; return; }
  M=$ADDR; C=$(qs $M '{"config":{}}' | jq -r .cw721_address)
  unlock "$tag submessage minter" $M val
  t "$tag submessage minter upsert_token_metadatas" ex $BIN val $M '{"upsert_token_metadatas":{"token_metadatas":[{"token_id":1,"metadata":{"name":"Locked 1"}}]}}'
  if ex $BIN val $M "{\"mint_for\":{\"token_id\":1,\"recipient\":\"$U1\"}}"; then ok "$tag submessage collection usable"
  elif why | grep -qi "migration in progress"; then
    known "$tag submessage collection usable: no, stays in migration (collection $C accepts migration_done only from the minter contract)"
  else bad "$tag submessage collection usable: $(why)"; fi
}

# cw4-group + cw3-flex-multisig (passage-2 codes 29 and 30) -> MS
multisig_suite() { # set group-code multisig-code
  local tag="$P/$1" pid r0
  MS=""
  inst $BIN $2 "$(jq -nc --arg v $VAL --arg a $U1 --arg b $U2 '{admin:$v,members:[{addr:$a,weight:1},{addr:$b,weight:1}]}')" group \
    && ok "$tag cw4-group instantiate" || { bad "$tag cw4-group instantiate: $(why)"; return; }
  local GRP=$ADDR
  inst $BIN $3 "$(jq -nc --arg g $GRP '{group_addr:$g,threshold:{absolute_count:{weight:2}},max_voting_period:{time:3600},executor:null,proposal_deposit:null}')" multisig \
    && ok "$tag cw3-flex-multisig instantiate" || { bad "$tag cw3-flex-multisig instantiate: $(why)"; return; }
  MS=$ADDR
  t  "$tag fund multisig" tx $BIN bank send val $MS 10000upasg
  t  "$tag propose bank send" ex $BIN u1 $MS "{\"propose\":{\"title\":\"pay\",\"description\":\"pay u3\",\"msgs\":[{\"bank\":{\"send\":{\"to_address\":\"$U3\",\"amount\":[$(coin 5000)]}}}],\"latest\":null}}"
  pid=$(echo "$LAST" | jq -r '[.events[].attributes[]|select(.key=="proposal_id")|.value][0]')
  tno "$tag execute before threshold rejected" "" ex $BIN u1 $MS "{\"execute\":{\"proposal_id\":$pid}}"
  t  "$tag vote" ex $BIN u2 $MS "{\"vote\":{\"proposal_id\":$pid,\"vote\":\"yes\"}}"
  r0=$(bal $U3)
  t  "$tag execute" ex $BIN u1 $MS "{\"execute\":{\"proposal_id\":$pid}}"
  [ "$(delta $U3 $r0)" = 5000 ] && ok "$tag funds sent by multisig" || bad "$tag funds sent by multisig: $(delta $U3 $r0)"
}

run_all() {
  local a="$P:art" m="$P:mn" n
  ART_FO=""; ART_FR=""; ART_MINTER=""; ART_COLL=""; ART_MK=""; ART_AU=""; ART_VA=""; MN_MS=""
  store_all
  social_suite art ${CODE[$a:follow]} ${CODE[$a:friend]};                  ART_FO=$FO; ART_FR=$FR
  royalty_suite art ${CODE[$a:royalty_group]}
  classic_suite art ${CODE[$a:minter]} ${CODE[$a:pg721]}
  nft_suite art ${CODE[$a:whitelist]} ${CODE[$a:pg721_metadata_onchain]} ${CODE[$a:minter_metadata_onchain]}
  ART_MINTER=$MINTER; ART_COLL=$COLL
  if [ -n "$ART_COLL" ]; then
    market_suite art ${CODE[$a:marketplace_v2]} $ART_COLL 5;               ART_MK=$MK
    auction_suite art ${CODE[$a:auction_english]} $ART_COLL 6;             ART_AU=$AU
    vault_suite art ${CODE[$a:nft_vault]} $ART_COLL 7 ${ART_FO:-$VAL};     ART_VA=$VA
  fi
  [ $MN = 1 ] || return 0
  nft_suite mn ${CODE[$m:28]} ${CODE[$m:5]} ${CODE[$m:2]} ${CODE[$m:1]}
  if [ -n "$COLL" ]; then
    market_suite mn-31 ${CODE[$m:31]} $COLL 5
    market_suite mn-27 ${CODE[$m:27]} $COLL 6
    market_suite mn-3  ${CODE[$m:3]}  $COLL 7
  fi
  submsg_lock_check mn ${CODE[$m:2]} ${CODE[$m:5]}
  for n in 1 5 32 33; do pg_suite mn-pg$n ${CODE[$m:$n]}; done
  multisig_suite mn ${CODE[$m:29]} ${CODE[$m:30]};                          MN_MS=$MS
  if [ -n "$ART_MINTER" ]; then
    t "$P/mn migrate minter to passage-2 code 2" tx $BIN wasm migrate $ART_MINTER ${CODE[$m:2]} '{}' --from val
    t "$P/mn migrate minter back to the artifact code" tx $BIN wasm migrate $ART_MINTER ${CODE[$a:minter_metadata_onchain]} '{}' --from val
  fi
}

snapshot() { # dir: every contract's raw state, sorted
  mkdir -p $1; local c code
  for code in $($NEW q wasm list-code --home $A -o json --limit 1000 | jq -r '.code_infos[].code_id'); do
    for c in $($NEW q wasm list-contract-by-code $code --home $A -o json --limit 1000 | jq -r '.contracts[]'); do
      $NEW q wasm contract-state all $c --home $A -o json --limit 100000 | jq -S '.models' > $1/$c.json
    done
  done
  ls $1 | wc -l; }

# ---------------------------------------------------------------- old binary
PA=$(start $OLD $A a_old ""); PB=$(start $OLD $B b_old "$BP")
waith 3 26657 && ok "old chain produces blocks" || { bad "old chain start"; tail -30 /n/a_old.log; exit 1; }

if [ "${QUICK:-0}" = 1 ]; then
  step "QUICK: scenarios on the new binary only"
  P=new; BIN=$NEW; run_all
  kill $PA $PB 2>/dev/null; cp /n/*.log "$OUT/" 2>/dev/null
  echo; echo "===== SUMMARY"; echo "PASS $(grep -c '^PASS' $OUT/summary.txt)  FAIL $(grep -c '^FAIL' $OUT/summary.txt)"
  grep -E '^(FAIL|KNOWN)' $OUT/summary.txt; exit 0
fi

step "scenarios on the old binary"
P=old; BIN=$OLD
run_all

step "in-flight state for the upgrade"
PEND=0
if [ -n "$ART_MK" ] && [ -n "$ART_AU" ] && [ -n "$ART_VA" ] && [ -n "$ART_FR" ]; then
  PEND=1
  t "pending: approve marketplace" approve u1 $ART_COLL $ART_MK 8
  t "pending: ask set" ex $BIN u1 $ART_MK "{\"set_ask\":{\"token_id\":\"8\",\"price\":$(coin 2000000),\"funds_recipient\":null}}"
  t "pending: offer on unlisted NFT" ex $BIN u2 $ART_MK "{\"set_bid\":{\"token_id\":\"11\",\"price\":$(coin 2500000)}}" --amount 2500000upasg
  t "pending: approve auction" approve u1 $ART_COLL $ART_AU 9
  T=$(now)
  t "pending: auction set" ex $BIN u1 $ART_AU "{\"set_auction\":{\"token_id\":\"9\",\"start_time\":\"$(ns $((T+8)))\",\"end_time\":\"$(ns $((T+150)))\",\"starting_price\":$(coin 1000),\"reserve_price\":$(coin 5000),\"funds_recipient\":null}}"
  until_t $((T+9))
  t "pending: auction bid" ex $BIN u2 $ART_AU "{\"set_auction_bid\":{\"token_id\":\"9\",\"price\":$(coin 1000)}}" --amount 1000upasg
  t "pending: approve vault" approve u1 $ART_COLL $ART_VA 10
  t "pending: stake" ex $BIN u1 $ART_VA '{"stake":{"token_id":"10"}}'
  t "pending: unstake" ex $BIN u1 $ART_VA '{"unstake":{"token_id":"10"}}'
  t "pending: friend request" ex $BIN u1 $ART_FR "{\"add_friend\":{\"target\":\"$U3\"}}"
  PEND_MS=$MN_MS; PEND_PID=""
  if [ -n "$PEND_MS" ]; then
    t "pending: multisig proposal" ex $BIN u1 $PEND_MS "{\"propose\":{\"title\":\"pending\",\"description\":\"pending\",\"msgs\":[{\"bank\":{\"send\":{\"to_address\":\"$U3\",\"amount\":[$(coin 3000)]}}}],\"latest\":null}}"
    PEND_PID=$(echo "$LAST" | jq -r '[.events[].attributes[]|select(.key=="proposal_id")|.value][0]')
  fi
  PEND_MK=$ART_MK; PEND_AU=$ART_AU; PEND_VA=$ART_VA; PEND_FR=$ART_FR; PEND_COLL=$ART_COLL; PEND_MINTER=$ART_MINTER; PEND_FO=$ART_FO
fi

step "software upgrade $UPG"
GOV=$($OLD q auth module-account gov --home $A -o json | jq -r '[..|.address?|select(.!=null)][0]')
H=$(( $(height 26657) + 25 ))
jq -n --arg gov "$GOV" --arg n "$UPG" --argjson h $H '{messages:[{"@type":"/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",authority:$gov,plan:{name:$n,height:($h|tostring),info:""}}],deposit:"2000000upasg",title:"upgrade",summary:"upgrade",expedited:true}' > /tmp/prop.json
t "submit upgrade proposal" tx $OLD gov submit-proposal /tmp/prop.json --from val
PID=$($OLD q gov proposals --home $A -o json | jq -r '.proposals[-1].id')
GAS="--gas 400000" t "vote upgrade proposal" tx $OLD gov vote $PID yes --from val
waith $((H-1)) 26657; sleep 8
grep -q "UPGRADE \"$UPG\" NEEDED" /n/a_old.log && ok "old binary halts at the upgrade height" || bad "old binary did not halt"
echo "contracts before upgrade: $(snapshot /n/snap_old)"
kill $PA $PB 2>/dev/null; sleep 3

# ---------------------------------------------------------------- new binary
step "new binary"
PA=$(start $NEW $A a_new ""); PB=$(start $NEW $B b_new "$BP")
waith $((H+5)) 26657 && ok "new binary produces blocks after the upgrade" || { bad "new binary after upgrade"; tail -40 /n/a_new.log; }
waith $((H+5)) 36657 && ok "full node follows after the upgrade" || bad "full node after upgrade"
echo "contracts after upgrade: $(snapshot /n/snap_new)"
diff -r /n/snap_old /n/snap_new > /n/snap.diff && ok "raw state of every contract identical after the upgrade" || { bad "contract state changed by the upgrade"; head -40 /n/snap.diff; }

P=new; BIN=$NEW
if [ $PEND = 1 ]; then
  step "complete in-flight state on the new binary"
  t  "pending: buy NFT listed before the upgrade" ex $BIN u2 $PEND_MK "{\"set_bid\":{\"token_id\":\"8\",\"price\":$(coin 2000000)}}" --amount 2000000upasg
  [ "$(owner $PEND_COLL 8)" = "$U2" ] && ok "pending: buyer owns NFT" || bad "pending: buyer owns NFT"
  t  "pending: approve for offer" approve u1 $PEND_COLL $PEND_MK 11
  t  "pending: accept offer made before the upgrade" ex $BIN u1 $PEND_MK "{\"accept_bid\":{\"token_id\":\"11\",\"bidder\":\"$U2\"}}"
  [ "$(owner $PEND_COLL 11)" = "$U2" ] && ok "pending: offer bidder owns NFT" || bad "pending: offer bidder owns NFT"
  b0=$(bal $U2)
  t  "pending: outbid auction bid made before the upgrade" ex $BIN u3 $PEND_AU "{\"set_auction_bid\":{\"token_id\":\"9\",\"price\":$(coin 6000)}}" --amount 6000upasg
  [ "$(delta $U2 $b0)" = 1000 ] && ok "pending: pre-upgrade bidder refunded" || bad "pending: pre-upgrade bidder refunded: $(delta $U2 $b0)"
  t  "pending: withdraw NFT unstaked before the upgrade" ex $BIN u1 $PEND_VA '{"withdraw":{"token_id":"10"}}'
  [ "$(owner $PEND_COLL 10)" = "$U1" ] && ok "pending: NFT back from vault" || bad "pending: NFT back from vault"
  t  "pending: accept friend request made before the upgrade" ex $BIN u3 $PEND_FR "{\"accept_friend_request\":{\"origin\":\"$U1\"}}"
  qc "pending: friends" $PEND_FR "{\"is_friend\":{\"origin\":\"$U1\",\"target\":\"$U3\"}}" '.is_friend==true'
  t  "pending: public mint on a minter instantiated before the upgrade" ex $BIN u3 $PEND_MINTER '{"mint":{}}' --amount 1000000upasg
  if [ -n "$PEND_PID" ]; then
    t "pending: vote multisig proposal made before the upgrade" ex $BIN u2 $PEND_MS "{\"vote\":{\"proposal_id\":$PEND_PID,\"vote\":\"yes\"}}"
    t "pending: execute multisig proposal" ex $BIN u1 $PEND_MS "{\"execute\":{\"proposal_id\":$PEND_PID}}"
  fi
  t  "amino-json (Ledger) MsgExecuteContract on follow" ex $NEW u3 $PEND_FO "{\"add_follow\":{\"target\":\"$U1\"}}" --sign-mode amino-json
  t  "amino-json (Ledger) cw721 transfer" ex $NEW u1 $PEND_COLL "{\"transfer_nft\":{\"recipient\":\"$U3\",\"token_id\":\"10\"}}" --sign-mode amino-json
  end=$(qs $PEND_AU '{"auction":{"token_id":"9"}}' | jq -r '.auction.end_time|tonumber/1000000000|floor')
  until_t $((end+1))
  t  "pending: finalize auction started before the upgrade" ex $BIN val $PEND_AU '{"finalize_auction":{"token_id":"9"}}'
  [ "$(owner $PEND_COLL 9)" = "$U3" ] && ok "pending: auction winner owns NFT" || bad "pending: auction winner owns NFT"
fi

step "scenarios on the new binary (fresh code)"
run_all

H2=$(( $(height 26657) + 3 )); waith $H2 26657; waith $H2 36657
[ "$(curl -s "localhost:26657/block?height=$H2" | jq -r .result.block.header.app_hash)" = "$(curl -s "localhost:36657/block?height=$H2" | jq -r .result.block.header.app_hash)" ] \
  && ok "app_hash equal on validator and full node" || bad "app_hash differs between nodes"
grep -qiE 'CONSENSUS FAILURE|wrong Block.Header.AppHash|^panic' /n/a_new.log /n/b_new.log && bad "consensus failure or panic in logs" || ok "no consensus failure or panic in logs"
kill $PA $PB 2>/dev/null
cp /n/*.log "$OUT/" 2>/dev/null

step "old vs new"
awk '
  /^(PASS|FAIL|KNOWN): (old|new)\// {
    s = $1; sub(/:$/, "", s); rest = substr($0, length($1) + 2)
    ph = substr(rest, 1, 3); name = substr(rest, 5); sub(/:.*/, "", name)
    r[ph, name] = s; names[name] = 1 }
  END {
    for (n in names) {
      o = r["old", n]; w = r["new", n]
      if (o == "PASS" && w != "PASS") print "REGRESSION: " n " (old PASS, new " (w == "" ? "missing" : w) ")"
      else if (o != "PASS" && o != "" && w == "PASS") print "FIXED: " n
      else if (o == "FAIL" && w == "FAIL") print "BOTH FAIL: " n
    } }' "$OUT/summary.txt" | sort | tee "$OUT/compare.txt"
grep -q '^REGRESSION' "$OUT/compare.txt" && bad "comparison of old and new outcomes: regressions found" || ok "comparison of old and new outcomes: no regression"

echo; echo "===== SUMMARY"
echo "PASS $(grep -c '^PASS' $OUT/summary.txt)  FAIL $(grep -c '^FAIL' $OUT/summary.txt)  KNOWN $(grep -c '^KNOWN' $OUT/summary.txt)"
grep -E '^(FAIL|KNOWN)' $OUT/summary.txt
