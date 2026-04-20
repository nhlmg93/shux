#!/usr/bin/env bash
set -euo pipefail

PREFIX_INPUT="${1:-$(pwd)/ghostty-build/usr}"
GHOSTTY_REPO="${GHOSTTY_REPO:-https://github.com/ghostty-org/ghostty.git}"
GHOSTTY_REF="${GHOSTTY_REF:-dcc39dcd401975ee77a642fa15ba7bb9f6d85b96}"

mkdir -p "$PREFIX_INPUT"
PREFIX="$(cd "$PREFIX_INPUT" && pwd)"
BUILD_ROOT="$(dirname "$PREFIX")"
STAMP_FILE="$PREFIX/.ghostty-ref"
LIB_PATH="$PREFIX/lib/libghostty-vt-static.a"
PC_PATH="$PREFIX/lib/pkgconfig/libghostty-vt-static.pc"
INCLUDE_DIR="$PREFIX/include"

if [[ -f "$LIB_PATH" && -f "$PC_PATH" && -f "$STAMP_FILE" ]] && [[ "$(cat "$STAMP_FILE")" == "$GHOSTTY_REF" ]]; then
	echo "ghostty-vt: using cached build for $GHOSTTY_REF"
	exit 0
fi

SRC_DIR="$BUILD_ROOT/.ghostty-src-$GHOSTTY_REF"
rm -rf "$SRC_DIR"
mkdir -p "$SRC_DIR" "$PREFIX/lib/pkgconfig" "$INCLUDE_DIR"
trap 'rm -rf "$SRC_DIR"' EXIT

echo "ghostty-vt: building ref $GHOSTTY_REF"

git init -q "$SRC_DIR"
git -C "$SRC_DIR" remote add origin "$GHOSTTY_REPO"
git -C "$SRC_DIR" fetch --depth=1 origin "$GHOSTTY_REF"
git -C "$SRC_DIR" checkout -q FETCH_HEAD

(
	cd "$SRC_DIR"
	zig build -Doptimize=ReleaseFast -Demit-lib-vt=true
)

cp "$SRC_DIR/zig-out/lib/libghostty-vt.a" "$LIB_PATH"
rm -rf "$INCLUDE_DIR/ghostty"
cp -r "$SRC_DIR/include/ghostty" "$INCLUDE_DIR/"
printf '%s\n' \
  "prefix=$PREFIX" \
  'exec_prefix=${prefix}' \
  'libdir=${prefix}/lib' \
  'includedir=${prefix}/include' \
  '' \
  'Name: libghostty-vt-static' \
  'Description: Ghostty VT static library' \
  'Version: 0' \
  'Libs: -L${libdir} -lghostty-vt-static' \
  'Cflags: -I${includedir}' \
  > "$PC_PATH"
printf '%s\n' "$GHOSTTY_REF" > "$STAMP_FILE"

echo "ghostty-vt: built $LIB_PATH"
