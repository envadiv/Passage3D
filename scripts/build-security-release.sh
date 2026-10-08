#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
version=${VERSION:-v4.1.0-rc1}
out=${OUT_DIR:-build/security-release}
arch=${GOARCH:-amd64}
case "$arch" in
  amd64) asset=libwasmvm_muslc.x86_64.a; checksum=56e7c590fe11a6a51381c80c2710f71af1244acfb8cd1d5839d638313b7bd401 ;;
  arm64) asset=libwasmvm_muslc.aarch64.a; checksum=4b632c22534d330b5111d7279faa7c96374e2097cbdff208cc5e040f1df41982 ;;
  *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
esac

test "$(go list -m -f '{{.Version}}' github.com/CosmWasm/wasmd)" = v0.54.10
test "$(go list -m -f '{{.Version}}' github.com/CosmWasm/wasmvm/v2)" = v2.2.9
mkdir -p "$out/lib/$arch"
out=$(cd "$out" && pwd)
lib="$out/lib/$arch/$asset"
curl --fail --location --retry 3 \
  "https://github.com/CosmWasm/wasmvm/releases/download/v2.2.9/$asset" -o "$lib"
printf '%s  %s\n' "$checksum" "$lib" | sha256sum --check

commit=$(git rev-parse HEAD)
dirty=$(git status --porcelain --untracked-files=normal)
if [[ -n "$dirty" && "$version" != *-rc* ]]; then
  echo "A final release requires a clean committed checkout" >&2
  exit 1
fi
binary="$out/passage-${version#v}-linux-$arch"
export CGO_ENABLED=1 GOOS=linux GOARCH="$arch"
export CC=${CC:-musl-gcc}
export CGO_LDFLAGS="-L$out/lib/$arch"
go build -p "${BUILD_PARALLELISM:-8}" -mod=readonly -trimpath -tags netgo,muslc \
  -ldflags "-s -w -X github.com/cosmos/cosmos-sdk/version.Name=passage -X github.com/cosmos/cosmos-sdk/version.AppName=passage -X github.com/cosmos/cosmos-sdk/version.Version=$version -X github.com/cosmos/cosmos-sdk/version.Commit=$commit -X github.com/cosmos/cosmos-sdk/version.BuildTags=netgo,muslc -linkmode=external -extldflags '-static -lm'" \
  -o "$binary" ./cmd/passage

if [[ "$arch" = "$(go env GOHOSTARCH)" ]]; then
  actual=$("$binary" query wasm libwasmvm-version)
  test "$actual" = 2.2.9 || { echo "Runtime wasmvm mismatch: $actual" >&2; exit 1; }
fi
(
  cd "$out"
  sha256sum "$(basename "$binary")" > "SHA256SUMS-$arch.txt"
)
printf 'version=%s\nplan=v4.1.0\ncommit=%s\nwasmd=v0.54.10\nwasmvm=v2.2.9\ngo=%s\ndirty=%s\n' \
  "$version" "$commit" "$(go version)" "$([[ -n "$dirty" ]] && echo true || echo false)" \
  > "$binary.provenance.txt"
echo "Built $binary"
