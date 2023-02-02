export PATH=/workspace/Passage3D/build:$PATH

DIR_PATH=/workspace/Passage3D

make build-linux

passage config chain-id localpassage

$DIR_PATH/tests/localpassage/scripts/add-keys.sh