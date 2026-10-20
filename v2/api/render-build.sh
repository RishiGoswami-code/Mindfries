#!/usr/bin/env bash
# Render build for the V2 engine: cross-compile the mindfries CLI for every
# target (API URL baked in, trusting the engine's signing key for the pilot),
# then build the server. The CLI builds land in dist/, which the server hosts at
# /install.sh + /cli/latest when V2_DIST_DIR=dist.
#
#   Render → Root Directory: v2/api
#            Build Command:  bash render-build.sh
#            Start Command:  ./bin/server
#            Env: V2_DIST_DIR=dist, V2_PUBLIC_URL=https://mindfries-cli.onrender.com
#                 (plus V2_DATABASE_URL, V2_ADMIN_TOKEN)
set -euo pipefail
cd "$(dirname "$0")" # v2/api

DIST="$(pwd)/dist"
mkdir -p "$DIST"

BASE_URL="${V2_PUBLIC_URL:-https://mindfries-cli.onrender.com}"
API_URL="${BASE_URL%/}/v2/cli"
PKG=github.com/mindfries/v2/cli/internal/buildinfo
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
# TrustDevKey=1: pilot builds accept the engine's dev signing key from its own
# host. For production, drop it and configure V2_SIGNING_* + ManifestKeys.
LD="-s -w -X ${PKG}.Version=0.1.0 -X ${PKG}.Commit=${COMMIT} -X ${PKG}.DefaultAPI=${API_URL} -X ${PKG}.TrustDevKey=1"

echo "building CLI (API=${API_URL}) into ${DIST}"
(
  cd ../cli
  export CGO_ENABLED=0
  for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
    os="${t%/*}"; arch="${t#*/}"
    GOOS="$os" GOARCH="$arch" go build -ldflags "$LD" -o "${DIST}/mindfries_${os}_${arch}" ./cmd/mindfries
  done
  GOOS=windows GOARCH=amd64 go build -ldflags "$LD" -o "${DIST}/mindfries_windows_amd64.exe" ./cmd/mindfries
)

echo "building server"
go build -o bin/server ./cmd/server
echo "done: $(ls "$DIST")"
