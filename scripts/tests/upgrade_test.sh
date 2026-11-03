#!/usr/bin/env bash
# In-place upgrade regression test, run in Docker:
#   OLD binary = OLD_REF (default eea1f3e = v4.0.3, the passage-2 mainnet release)
#   NEW binary = the current working tree
# A validator and a full node run the old binary, state is created (bank, staking,
# authz, feegrant, group, CosmWasm, claim), a software-upgrade proposal for $UPG is
# passed, both nodes switch to the new binary and every v4.1.0 fix is checked.
#
# Usage: scripts/tests/upgrade_test.sh            (or: make test-upgrade)
# Results: build/upgrade-test/{summary.txt,upgrade.log,*.log}
set -euo pipefail

# Git Bash on Windows: pass D:/... host paths and keep container paths unconverted.
export MSYS_NO_PATHCONV=1
abs() { (cd "$1" && (pwd -W 2>/dev/null || pwd)); }
ROOT=$(abs "$(dirname "$0")/../..")
OUT=${OUT:-$ROOT/build/upgrade-test}
mkdir -p "$OUT"; OUT=$(abs "$OUT")

docker run --rm \
  -e OLD_REF="${OLD_REF:-eea1f3e}" \
  -e UPG="${UPG:-v4.1.0}" \
  -e GOFLAGS=-buildvcs=false \
  -v "$ROOT":/src:ro \
  -v "$OUT":/out \
  ${GOMODCACHE_VOLUME:+-v "$GOMODCACHE_VOLUME":/go/pkg/mod} \
  ${GOCACHE_VOLUME:+-v "$GOCACHE_VOLUME":/root/.cache/go-build} \
  golang:1.22.11-bookworm bash /src/scripts/tests/upgrade_inner.sh

grep -q '^FAIL' "$OUT/summary.txt" && { echo "upgrade test FAILED"; exit 1; }
echo "upgrade test passed"
