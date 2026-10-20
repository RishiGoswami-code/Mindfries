#!/bin/sh
# Mindfries CLI installer for macOS, Linux and Git Bash on Windows.
#
# Install (the candidate runs this once):
#   curl -fsSL https://<v2-domain>/install.sh | sh
# then, in any terminal:
#   mindfries start
#
# It downloads the release's checksums, installs mindfries only if the
# binary's SHA-256 matches (a tampered binary is refused and deleted), puts
# it in ~/.local/bin and, if that folder isn't on PATH, adds it in the
# shell's startup file. A binary that already matches the release isn't
# downloaded again. Given a setup code, as a dashboard command can do:
#   curl -fsSL https://<v2-domain>/install.sh | sh -s -- <SETUP-CODE>
# it then runs `mindfries setup <SETUP-CODE>` (the dashboard serves this
# script with the code already in it), and you run `mindfries start` next.
#
# Environment (for testing and mirrors):
#   MINDFRIES_DOWNLOAD        release base URL, or a local folder (default below)
#   MINDFRIES_INSTALL_DIR     where the binary goes (default ~/.local/bin)
#   MINDFRIES_NO_MODIFY_PATH  1 leaves shell startup files alone
set -eu

BASE="${MINDFRIES_DOWNLOAD:-https://get.v2.mindfries.example/cli/latest}"
DIR="${MINDFRIES_INSTALL_DIR:-$HOME/.local/bin}"
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

if [ -d "$BASE" ]; then
  # A local folder works as the release too, for dev-install.sh.
  fetch() { cp "${1}" "$2"; }
elif command -v curl >/dev/null 2>&1; then
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

fetch "$BASE/checksums.txt" "$tmp/checksums.txt" || fail "couldn't download the release checksums from $BASE. Check your connection and run the install command again."
want="$(awk -v n="$name" '$2 == n || $2 == "*" n { print $1 }' "$tmp/checksums.txt")"
[ -n "$want" ] || fail "the release has no build for $os/$arch."

bin="$DIR/mindfries$ext"
if [ -f "$bin" ] && [ "$(hash "$bin")" = "$want" ]; then
  say "mindfries is up to date."
else
  say "Downloading mindfries for ${os}/${arch}..."
  fetch "$BASE/$name" "$tmp/$name" || fail "couldn't download mindfries. Check your connection and run the install command again."
  got="$(hash "$tmp/$name")"
  if [ "$got" != "$want" ]; then
    rm -f "$tmp/$name"
    fail "the download doesn't match the release checksum, so it was deleted and not run. Run the install command again; if this keeps happening, tell the hiring team."
  fi
  mkdir -p "$DIR"
  chmod +x "$tmp/$name"
  mv -f "$tmp/$name" "$bin"
  say "Installed $bin"
fi

# The startup file of the candidate's login shell, where PATH is set.
profile_file() {
  case "$(basename "${SHELL:-sh}")" in
    zsh) printf '%s\n' "${ZDOTDIR:-$HOME}/.zshrc" ;;
    bash)
      if [ "$os" = darwin ]; then printf '%s\n' "$HOME/.bash_profile"; else printf '%s\n' "$HOME/.bashrc"; fi ;;
    fish) printf '%s\n' "$HOME/.config/fish/conf.d/mindfries.fish" ;;
    *) printf '%s\n' "$HOME/.profile" ;;
  esac
}

marker="# Added by the Mindfries installer"
case ":$PATH:" in
  *":$DIR:"*) ;;
  *)
    if [ "${MINDFRIES_NO_MODIFY_PATH:-}" = 1 ]; then
      say "Add $DIR to your PATH to run 'mindfries' from any terminal."
    else
      rc="$(profile_file)"
      if [ -f "$rc" ] && grep -qF "$marker" "$rc"; then
        :
      else
        mkdir -p "$(dirname "$rc")"
        case "$rc" in
          *.fish) line="fish_add_path \"$DIR\"" ;;
          *) line="export PATH=\"$DIR:\$PATH\"" ;;
        esac
        printf '\n%s\n%s\n' "$marker" "$line" >> "$rc"
        say "Added $DIR to your PATH in $rc."
      fi
      say "Open a new terminal (or run: export PATH=\"$DIR:\$PATH\") so 'mindfries' is found."
    fi
    ;;
esac

if [ -z "$CODE" ]; then
  say ""
  say "Next, in a new terminal:  mindfries start"
  exit 0
fi
# The script itself arrives on stdin, so give mindfries the terminal.
if [ -r /dev/tty ] && (: </dev/tty) 2>/dev/null; then
  exec "$bin" setup "$CODE" </dev/tty
fi
exec "$bin" setup "$CODE"
