#!/bin/sh
# Builds mindfries from this checkout and installs it on your PATH through
# install.sh, exactly as a candidate's install would, so you can then type
# `mindfries start` in any terminal. For developers; needs Go.
#
#   sh v2/cli/install/dev-install.sh
#   sh v2/cli/install/dev-install.sh http://localhost:8787/v2/cli
#
# The argument is baked into the build (default: the fake API hosted on
# Render). The build also trusts the fake API's development signing key
# from that server, which only dev builds can do.
set -eu

API="${1:-https://mindfries-cli.onrender.com/v2/cli}"
here="$(cd "$(dirname "$0")" && pwd)"
cli="$(dirname "$here")"

case "$(uname -s)" in
  Linux) os=linux; ext="" ;;
  Darwin) os=darwin; ext="" ;;
  MINGW*|MSYS*|CYGWIN*) os=windows; ext=".exe" ;;
  *) echo "unsupported OS" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported processor" >&2; exit 1 ;;
esac
name="mindfries_${os}_${arch}${ext}"

dist="$(mktemp -d)"
trap 'rm -rf "$dist"' EXIT

commit="$(git -C "$cli" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"
pkg=github.com/mindfries/v2/cli/internal
echo "Building mindfries for $API..."
(cd "$cli" && CGO_ENABLED=0 go build -trimpath \
  -ldflags "-X $pkg/buildinfo.DefaultAPI=$API -X $pkg/buildinfo.TrustDevKey=1 -X $pkg/buildinfo.Commit=$commit" \
  -o "$dist/$name" ./cmd/mindfries)
if command -v sha256sum >/dev/null 2>&1; then
  sum="$(sha256sum < "$dist/$name" | cut -d' ' -f1)"
else
  sum="$(shasum -a 256 < "$dist/$name" | cut -d' ' -f1)"
fi
printf '%s  %s\n' "$sum" "$name" > "$dist/checksums.txt"

MINDFRIES_DOWNLOAD="$dist" sh "$here/install.sh"
