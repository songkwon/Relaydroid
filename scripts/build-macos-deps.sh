#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p build/native/sources
prefix="$(cd build/native && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-14.0}"

download() {
  local url="$1" archive="$2" sha="$3"
  curl --fail --location --retry 2 --output "$archive" "$url"
  printf '%s  %s\n' "$sha" "$archive" | shasum -a 256 -c
  tar -xf "$archive"
}

cd "$work"
download 'https://github.com/libusb/libusb/releases/download/v1.0.30/libusb-1.0.30.tar.bz2' libusb.tar.bz2 fea36f34f9156400209595e300840767ab1a385ede1dc7ee893015aea9c6dbaf
download 'https://downloads.sourceforge.net/project/libmtp/libmtp/1.1.23/libmtp-1.1.23.tar.gz' libmtp.tar.gz 74a2b6e8cb4a0304e95b995496ea3ac644c29371649b892b856e22f12a0bdeed
cp libusb.tar.bz2 "$prefix/sources/libusb-1.0.30.tar.bz2"
cp libmtp.tar.gz "$prefix/sources/libmtp-1.1.23.tar.gz"
cd libusb-1.0.30
./configure --prefix="$prefix/libusb" --disable-static
make -j 4
make install
cp COPYING "$prefix/libusb/COPYING"

cd "$work/libmtp-1.1.23"
export PKG_CONFIG_PATH="$prefix/libusb/lib/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
./configure --prefix="$prefix/libmtp" --disable-static
make -j 4
make install
cp COPYING "$prefix/libmtp/COPYING"
