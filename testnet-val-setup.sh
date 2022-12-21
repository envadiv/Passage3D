#!/bin/bash
set -e

command_exists () {
    type "$1" &> /dev/null ;
}

if command_exists go ; then
    echo "Golang is already installed"
else
  echo "Install dependencies"
  sudo apt update
  sudo apt install build-essential jq wget git -y

  wget https://dl.google.com/go/go1.17.1.linux-amd64.tar.gz
  tar -xvf go1.17.1.linux-amd64.tar.gz
  sudo mv go /usr/local

  echo "" >> ~/.bashrc
  echo 'export GOPATH=$HOME/go' >> ~/.bashrc
  echo 'export GOROOT=/usr/local/go' >> ~/.bashrc
  echo 'export GOBIN=$GOPATH/bin' >> ~/.bashrc
  echo 'export PATH=$PATH:/usr/local/go/bin:$GOBIN' >> ~/.bashrc
  
fi

source ~/.bashrc

echo "CAUTION!"
echo "-- If Passage3D was previously installed, the following step will remove ~/.passage from your system. Are you sure you would like to continue?--"

select yn in "Yes" "No"; do
    case $yn in
        Yes ) rm -rf ~/.passage; break;;
        No ) exit;;
    esac
done

DAEMON=passage
DENOM=upasg
CHAIN_ID=passage-1
PERSISTENT_PEERS=""

echo "install Passage3D"
git clone https://github.com/envadiv/Passage3D
cd Passage3D
git fetch
git checkout v1.0.0
make install

echo "Passage3D has been installed successfully!"
echo ""
echo "-- Next we will need to set up your keys and moniker"
echo "-- Please choose a name for your key --"
read YOUR_KEY_NAME

echo "-- Please choose a moniker --"
read YOUR_NAME

echo "-- Your Key Name is $YOUR_KEY_NAME and your moniker is $YOUR_NAME. Is this correct?"

select yn in "Yes" "No" "Cancel"; do
    case $yn in
        Yes ) break;;
        No ) echo "-- Please choose a name for your key --";
             read YOUR_KEY_NAME;
             echo "-- Please choose a moniker --";
             read YOUR_NAME; break;;
        Cancel ) exit;;
    esac
done

echo "-- Your Key Name is $YOUR_KEY_NAME and your moniker is $YOUR_NAME. --"

echo "Creating keys"
$DAEMON keys add $YOUR_KEY_NAME

echo ""
echo "After you have copied the mnemonic phrase in a safe place,"
echo "press the space bar to continue."
read -s -d ' '
echo ""

echo "----------Setting up your validator node------------"
$DAEMON init --chain-id $CHAIN_ID $YOUR_NAME
echo "------Downloading Passage3D Mainnet genesis--------"
curl -s https://raw.githubusercontent.com/envadiv/mainnet/main/passage-1/genesis.json > ~/.passage/config/genesis.json

echo "----------Setting config for seed node---------"
sed -i 's#tcp://127.0.0.1:26657#tcp://0.0.0.0:26657#g' ~/.$DAEMON/config/config.toml
sed -i '/persistent_peers =/c\persistent_peers = "'"$PERSISTENT_PEERS"'"' ~/.$DAEMON/config/config.toml

DAEMON_PATH=$(which $DAEMON)

echo "Installing cosmovisor - an upgrade manager..."

rm -rf $GOPATH/src/github.com/cosmos/cosmos-sdk
git clone https://github.com/cosmos/cosmos-sdk $GOPATH/src/github.com/cosmos/cosmos-sdk
cd $GOPATH/src/github.com/cosmos/cosmos-sdk
git checkout v0.45.0
cd cosmovisor
make cosmovisor
cp cosmovisor $GOBIN/cosmovisor

echo "Setting up cosmovisor directories"
mkdir -p ~/.passage/cosmovisor/genesis/bin
cp $GOBIN/passage ~/.passage/cosmovisor/genesis/bin

echo "---------Creating system file---------"

echo "[Unit]
Description=Cosmovisor daemon
After=network-online.target
[Service]
Environment="DAEMON_NAME=passage"
Environment="DAEMON_HOME=${HOME}/.${DAEMON}"
Environment="DAEMON_RESTART_AFTER_UPGRADE=on"
User=${USER}
ExecStart=${GOBIN}/cosmovisor start
Restart=always
RestartSec=3
LimitNOFILE=4096
[Install]
WantedBy=multi-user.target
" >cosmovisor.service

sudo mv cosmovisor.service /lib/systemd/system/cosmovisor.service
sudo -S systemctl daemon-reload
sudo -S systemctl start cosmovisor


echo ""
echo "--------------Congratulations---------------"
echo ""
echo "View your account address by typing your passphrase below." 
$DAEMON keys show $YOUR_KEY_NAME -a
echo ""
echo ""
echo "Next you will need to fund the above wallet address. When finished, you can create your validator by customizing and running the following command"
echo ""
echo "passage tx staking create-validator --amount 9000000000upasg --commission-max-change-rate \"0.1\" --commission-max-rate \"0.20\" --commission-rate \"0.1\" --details \"Some details about yourvalidator\" --from <keyname> --pubkey=\"$(passage tendermint show-validator)\" --moniker <your moniker> --min-self-delegation \"1\" --chain-id passage-1 --gas auto --fees 500upasg"