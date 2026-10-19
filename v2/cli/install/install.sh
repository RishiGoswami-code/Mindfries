#!/bin/sh
# Mindfries CLI installer for macOS, Linux and Git Bash on Windows.
#
# The dashboard's setup command runs it as:
#   curl -fsSL https://<v2-domain>/install.sh | sh -s -- <SETUP-CODE>
#
# It downloads the release's checksums, installs mindfries only if the
# binary's SHA-256 matches (a tampered binary is refused and deleted), then
# runs `mindfries start <SETUP-CODE>`. A binary that already matches the
# release isn't downloaded again.
#
# Environment (for testing and mirrors):
#   MINDFRIES_DOWNLOAD     release base URL (default below)
#   MINDFRIES_INSTALL_DIR  where the binary goes (default ~/.mindfries/bin)
set -eu

BASE="${MINDFRIES_DOWNLOAD:-https://get.v2.mindfries.example/cli/latest}"
DIR="${MINDFRIES_INSTALL_DIR:-$HOME/.mindfries/bin}"
CODE="${1:-}"

say() { printf '%s\n' "$*"; }
fail() { printf 'mindfries installer: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux; ext="" ;;
  Darwin) os=darwin; ext="" ;;
  MINGW*|MSYS*|CYGWIN*) os=windows; ext=".exe" ;;
  *) fail "this operating system isn't supported. Contact the hiring team before your assessment for another way to take it." ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) fail "this processor ($(uname -m)) isn't supported. Contact the hiring team before your assessment." ;;
esac
name="mindfries_${os}_${arch}${ext}"

if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL --retry 3 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -q -O "$2" "$1"; }
else
  fail "curl or wget is needed to download mindfries."
fi
if command -v sha256sum >/dev/null 2>&1; then
  hash() { sha256sum < "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
  hash() { shasum -a 256 < "$1" | cut -d' ' -f1; }
else
  fail "sha256sum or shasum is needed to check the download."
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fetch "$BASE/checksums.txt" "$tmp/checksums.txt" || fail "couldn't download the release checksums from $BASE. Check your connection and run the setup command again."
want="$(awk -v n="$name" '$2 == n || $2 == "*" n { print $1 }' "$tmp/checksums.txt")"
[ -n "$want" ] || fail "the release has no build for $os/$arch."

bin="$DIR/mindfries$ext"
if [ -f "$bin" ] && [ "$(hash "$bin")" = "$want" ]; then
  say "mindfries is up to date."
else
  say "Downloading mindfries for $os/$arch…"
  fetch "$BASE/$name" "$tmp/$name" || fail "couldn't download mindfries. Check your connection and run the setup command again."
  got="$(hash "$tmp/$name")"
  if [ "$got" != "$want" ]; then
    rm -f "$tmp/$name"
    fail "the download doesn't match the release checksum, so it was deleted and not run. Run the setup command again; if this keeps happening, tell the hiring team."
  fi
  mkdir -p "$DIR"
  chmod +x "$tmp/$name"
  mv -f "$tmp/$name" "$bin"
  say "Installed $bin"
fi

case ":$PATH:" in
  *":$DIR:"*) ;;
  *) say "Tip: add $DIR to your PATH to run 'mindfries' from any terminal." ;;
esac

[ -n "$CODE" ] || exit 0
# The script itself arrives on stdin, so give mindfries the terminal.
if [ -r /dev/tty ] && (: </dev/tty) 2>/dev/null; then
  exec "$bin" start "$CODE" </dev/tty
fi
exec "$bin" start "$CODE"
