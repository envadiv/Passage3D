#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
version=${VERSION:-v4.1.0-rc1}
out=${OUT_DIR:-build/security-release}
arch=${GOARCH:-amd64}
case "$arch" in
  amd64) asset=libwasmvm.x86_64.so; checksum=bfd7157b96028037eb1bdf82b2b7cbf4ebd38a5ab6b1336f0d35c73cf569c783 ;;
  arm64) asset=libwasmvm.aarch64.so; checksum=248f92469e65eb0d334247287c5d68035873e4c89fb30fe49fe006c3bf4c97b6 ;;
  *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
esac

test "$(go list -m -f '{{.Version}}' github.com/CosmWasm/wasmd)" = v0.54.10
test "$(go list -m -f '{{.Version}}' github.com/CosmWasm/wasmvm/v2)" = v2.2.9
mkdir -p "$out"
out=$(cd "$out" && pwd)
lib="$out/$asset"
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
export CC=${CC:-gcc}
export CGO_LDFLAGS='-Wl,-rpath,$ORIGIN'
go build -p "${BUILD_PARALLELISM:-8}" -mod=readonly -trimpath -tags netgo \
  -ldflags "-s -w -X github.com/cosmos/cosmos-sdk/version.Name=passage -X github.com/cosmos/cosmos-sdk/version.AppName=passage -X github.com/cosmos/cosmos-sdk/version.Version=$version -X github.com/cosmos/cosmos-sdk/version.Commit=$commit -X github.com/cosmos/cosmos-sdk/version.BuildTags=netgo -linkmode=external" \
  -o "$binary" ./cmd/passage

if [[ "$arch" = "$(go env GOHOSTARCH)" ]]; then
  actual=$("$binary" query wasm libwasmvm-version)
  test "$actual" = 2.2.9 || { echo "Runtime wasmvm mismatch: $actual" >&2; exit 1; }
fi
(
  cd "$out"
  sha256sum "$(basename "$binary")" "$asset" > "SHA256SUMS-$arch.txt"
)
printf 'version=%s\nplan=v4.1.0\ncommit=%s\nwasmd=v0.54.10\nwasmvm=v2.2.9\ngo=%s\ndirty=%s\n' \
  "$version" "$commit" "$(go version)" "$([[ -n "$dirty" ]] && echo true || echo false)" \
  > "$binary.provenance.txt"
tar -czf "$out/passage-${version#v}-linux-$arch.tar.gz" -C "$out" "$(basename "$binary")" "$asset" "$(basename "$binary").provenance.txt" "SHA256SUMS-$arch.txt"
echo "Built $binary and matching runtime bundle"
