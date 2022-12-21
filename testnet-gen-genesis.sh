#!/bin/sh
PASSAGE_HOME="/tmp/passage$(date +%s)"
CHAIN_ID=passage-1
DENOM=upasg

set -e

echo "...........Init Passage3D.............."

git clone https://github.com/envadiv/Passage3D
cd Passage3D
git checkout shlok/new-cosmwasm
make build
chmod +x ./build/passage

./build/passage init --chain-id $CHAIN_ID validator --home $PASSAGE_HOME

echo "..........Fetching genesis......."
rm -rf $PASSAGE_HOME/config/genesis.json
cp ../$CHAIN_ID/genesis-prelaunch.json $PASSAGE_HOME/config/genesis.json

sed -i "s/172800000000000/600000000000/g" $DAEMON_HOME-1/config/genesis.json
sed -i "s/172800s/600s/g" $DAEMON_HOME-1/config/genesis.json
sed -i "s/stake/$DENOM/g" $DAEMON_HOME-1/config/genesis.json

echo "..........Collecting gentxs......."
./build/passage collect-gentxs --home $PASSAGE_HOME --gentx-dir ../$CHAIN_ID/gentxs

./build/passage validate-genesis --home $PASSAGE_HOME

cp $PASSAGE_HOME/config/genesis.json ../$CHAIN_ID/genesis.json
jq -S -c -M '' ../$CHAIN_ID/genesis.json | shasum -a 256 > ../$CHAIN_ID/checksum.txt

echo "..........Starting node......."
./build/passage start --home $PASSAGE_HOME &

sleep 5s

echo "...Cleaning the stuff..."
killall passage >/dev/null 2>&1
rm -rf $PASSAGE_HOME >/dev/null 2>&1

cd ..
rm -rf Passage3D