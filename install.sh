#!/usr/bin/env bash
# install.sh — install the tharness release binary on Linux (Ubuntu/Debian/etc).
#
# Usage:
#   ./install.sh                 # installs to /usr/local/bin (sudo if needed)
#   PREFIX=~/.local/bin ./install.sh
#   THARNESS_VERSION=0.0.1 ./install.sh
#
# Downloads the release asset and checksums.txt from GitHub Releases,
# verifies the sha256, and installs the binary as `tharness`.
set -euo pipefail

REPO="Amirhossein-Mirzaei23/Tharness"
VERSION="${THARNESS_VERSION:-0.0.1}"
PREFIX="${PREFIX:-/usr/local/bin}"

case "$(uname -m)" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "install: unsupported arch: $(uname -m)" >&2; exit 1 ;;
esac

ASSET="tharness_${VERSION}_linux_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "install: downloading ${ASSET}"
curl -fsSL -o "$TMP/$ASSET" "${BASE_URL}/${ASSET}" ||
  { echo "install: download failed — is release v${VERSION} published?" >&2; exit 1; }
curl -fsSL -o "$TMP/checksums.txt" "${BASE_URL}/checksums.txt"

echo "install: verifying checksum"
WANT="$(grep " ${ASSET}\$" "$TMP/checksums.txt" | awk '{print $1}')"
[ -n "$WANT" ] || { echo "install: ${ASSET} missing from checksums.txt" >&2; exit 1; }
echo "${WANT}  $TMP/$ASSET" | sha256sum -c - >/dev/null ||
  { echo "install: checksum mismatch" >&2; exit 1; }

tar -xzf "$TMP/$ASSET" -C "$TMP"

INSTALL_CMD=(install -m 0755 "$TMP/tharness" "$PREFIX/tharness")
if [ ! -w "$PREFIX" ] && [ "$(id -u)" != 0 ]; then
  INSTALL_CMD=(sudo "${INSTALL_CMD[@]}")
fi
mkdir -p "$PREFIX"
"${INSTALL_CMD[@]}"

echo "install: tharness ${VERSION} installed to ${PREFIX}/tharness"
"${PREFIX}/tharness" --version
echo "next: run \`tharness setup\` then \`tharness\`"
