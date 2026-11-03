#!/usr/bin/env bash
# Generates the gogoproto Go code for the Passage protobuf definitions.
# Run it through `make proto-gen`, which uses the ghcr.io/cosmos/proto-builder image
# (buf + protoc-gen-gocosmos + protoc-gen-grpc-gateway).
set -eo pipefail

echo "Generating gogo proto code"
cd proto
proto_dirs=$(find ./passage3d -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  for file in $(find "${dir}" -maxdepth 1 -name '*.proto'); do
    if grep -q "option go_package" "$file"; then
      buf generate --template buf.gen.gogo.yaml "$file"
    fi
  done
done
cd ..

# move generated files to the right places
cp -r github.com/envadiv/Passage3D/* ./
rm -rf github.com
