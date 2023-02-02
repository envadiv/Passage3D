#!/bin/sh

CHAIN_ID=localpassage
PASSAGE_HOME=$HOME/.passage
CONFIG_FOLDER=$PASSAGE_HOME/config
MONIKER=lo-val

MNEMONIC="bottom loan skill merry east cradle onion journey palm apology verb edit desert impose absurd oil bubble sweet glove shallow size build burst effort"

install_prerequisites () {
    apk add dasel
}

edit_genesis () {

    GENESIS=$CONFIG_FOLDER/genesis.json

    # Update staking module
    dasel put string -f $GENESIS '.app_state.staking.params.bond_denom' 'upasg'
    dasel put string -f $GENESIS '.app_state.staking.params.unbonding_time' '240s'

    # Update crisis module
    dasel put string -f $GENESIS '.app_state.crisis.constant_fee.denom' 'upasg'

    # Udpate gov module
    dasel put string -f $GENESIS '.app_state.gov.voting_params.voting_period' '60s'
    dasel put string -f $GENESIS '.app_state.gov.deposit_params.min_deposit.[0].denom' 'upasg'

    # Update txfee basedenom
    dasel put string -f $GENESIS '.app_state.txfees.basedenom' "upasg"

    # Update wasm permission (Nobody or Everybody)
    dasel put string -f $GENESIS '.app_state.wasm.params.code_upload_access.permission' "Everybody"
}

add_genesis_accounts () {

    passage add-genesis-account pasg12smx2wdlyttvyzvzg54y2vnqwq2qjateh25pvl 100000000000upasg,100000000000stake
    passage add-genesis-account pasg1cyyzpxplxdzkeea7kwsydadg87357qnau7uhda 100000000000upasg,100000000000stake
    passage add-genesis-account pasg18s5lynnmx37hq4wlrw9gdn68sg2uxp5rr4qshp 100000000000upasg,100000000000stake
    passage add-genesis-account pasg1qwexv7c6sm95lwhzn9027vyu2ccneaqaxkydds 100000000000upasg,100000000000stake
    passage add-genesis-account pasg14hcxlnwlqtq75ttaxf674vk6mafspg8x9tee0u 100000000000upasg,100000000000stake
    passage add-genesis-account pasg12rr534cer5c0vj53eq4y32lcwguyy7nnxg9k3x 100000000000upasg,100000000000stake
    passage add-genesis-account pasg1nt33cjd5auzh36syym6azgc8tve0jlvk5s25fd 100000000000upasg,100000000000stake
    passage add-genesis-account pasg10qfrpash5g2vk3hppvu45x0g860czur8z27waz 100000000000upasg,100000000000stake
    passage add-genesis-account pasg1f4tvsdukfwh6s9swrc24gkuz23tp8pd3jxf7js 100000000000upasg,100000000000stake
    passage add-genesis-account pasg1myv43sqgnj5sm4zl98ftl45af9cfzk7nu3vcm6 100000000000upasg,100000000000stake
    passage add-genesis-account pasg14gs9zqh8m49yy9kscjqu9h72exyf295aztsunm 100000000000upasg,100000000000stake

    passage gentx $MONIKER 500000000upasg --keyring-backend=test --chain-id=$CHAIN_ID

    passage collect-gentxs
}

edit_config () {
    # Remove seeds
    dasel put string -f $CONFIG_FOLDER/config.toml '.p2p.seeds' ''

    # Expose the rpc
    dasel put string -f $CONFIG_FOLDER/config.toml '.rpc.laddr' "tcp://0.0.0.0:26657"
}

echo $MNEMONIC | passage init $MONIKER -o --chain-id=$CHAIN_ID --recover
install_prerequisites
edit_genesis
add_genesis_accounts
edit_config

passage start
