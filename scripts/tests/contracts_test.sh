#!/usr/bin/env bash
# CosmWasm contract regression test, run in Docker (see contracts_inner.sh for the scenarios):
#   contracts = passage-contracts artifacts (legacy contracts excluded) + passage-2 code
#   OLD binary = OLD_REF (default eea1f3e = v4.0.3), NEW binary = the current working tree
#
# Usage: scripts/tests/contracts_test.sh          (or: make test-contracts)
#   CONTRACTS_DIR  passage-contracts/artifacts (default ../passage-workspace/passage-contracts/artifacts)
#   MAINNET_DIR    optional directory with passage-2 code as code_<id>.wasm (default: download over gRPC)
#   MAINNET_GRPC   gRPC endpoint used for the download (default passage-grpc.stakeandrelax.net:15690)
#   QUICK=1        new binary only, no upgrade (fast check of the scenarios)
# Results: build/contracts-test/{summary.txt,compare.txt,contracts.log,*.log}
set -euo pipefail

# Git Bash on Windows: pass D:/... host paths and keep container paths unconverted.
export MSYS_NO_PATHCONV=1
abs() { (cd "$1" && (pwd -W 2>/dev/null || pwd)); }
ROOT=$(abs "$(dirname "$0")/../..")
OUT=${OUT:-$ROOT/build/contracts-test}
mkdir -p "$OUT"; OUT=$(abs "$OUT")
CONTRACTS_DIR=$(abs "${CONTRACTS_DIR:-$ROOT/../passage-workspace/passage-contracts/artifacts}")
[ -n "${MAINNET_DIR:-}" ] && MAINNET_DIR=$(abs "$MAINNET_DIR")
[ -f "$CONTRACTS_DIR/checksums.txt" ] || { echo "CONTRACTS_DIR=$CONTRACTS_DIR has no checksums.txt"; exit 1; }

docker run --rm \
  -e OLD_REF="${OLD_REF:-eea1f3e}" \
  -e UPG="${UPG:-v4.1.0}" \
  -e GOFLAGS=-buildvcs=false \
  ${QUICK:+-e QUICK="$QUICK"} \
  ${MAINNET_GRPC:+-e MAINNET_GRPC="$MAINNET_GRPC"} \
  ${MAINNET_RPC:+-e MAINNET_RPC="$MAINNET_RPC"} \
  -v "$ROOT":/src:ro \
  -v "$CONTRACTS_DIR":/contracts:ro \
  ${MAINNET_DIR:+-v "$MAINNET_DIR":/mainnet:ro} \
  -v "$OUT":/out \
  ${GOMODCACHE_VOLUME:+-v "$GOMODCACHE_VOLUME":/go/pkg/mod} \
  ${GOCACHE_VOLUME:+-v "$GOCACHE_VOLUME":/root/.cache/go-build} \
  golang:1.22.11-bookworm bash /src/scripts/tests/contracts_inner.sh

grep -q '^FAIL' "$OUT/summary.txt" && { echo "contracts test FAILED"; exit 1; }
echo "contracts test passed"
