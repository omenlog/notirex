#!/bin/sh
# Install notirex from its latest GitHub release.
set -eu

repo="omenlog/notirex"
version="${1:-latest}"
prefix="${NOTIREX_INSTALL_DIR:-$HOME/.local/bin}"

if [ "$(uname -s)" != "Darwin" ]; then
  echo "notirex currently supports macOS only." >&2
  exit 1
fi

case "$(uname -m)" in
  arm64) arch="arm64" ;;
  x86_64) arch="x86_64" ;;
  *)
    echo "notirex does not support $(uname -m) macOS." >&2
    exit 1
    ;;
esac

case "$version" in
  latest) release="latest" ;;
  v*) release="$version" ;;
  *) release="v$version" ;;
esac

archive="notirex_Darwin_${arch}.tar.gz"
url="https://github.com/${repo}/releases/download/${release}/${archive}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

curl --fail --location --silent --show-error "$url" --output "$tmpdir/$archive"
tar -xzf "$tmpdir/$archive" -C "$tmpdir"

mkdir -p "$prefix"
install -m 755 "$tmpdir/notirex" "$prefix/notirex"
printf 'Installed notirex to %s/notirex\n' "$prefix"

case ":$PATH:" in
  *":$prefix:"*) ;;
  *) echo "Add $prefix to PATH to invoke notirex." ;;
esac
