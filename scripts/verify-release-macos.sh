#!/bin/bash
set -euo pipefail

if [[ $# != 2 || ! "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ || "$(uname -s)" != Darwin ]]; then
  echo "Usage on macOS: $0 vMAJOR.MINOR.PATCH OUTPUT_CHECKSUMS" >&2
  exit 2
fi

tag="$1"
checksums="$2"
repo=songkwon/relaydroid
assets="$(mktemp -d)"
trap 'rm -rf "$assets"' EXIT

test "$(gh release view "$tag" --repo "$repo" --json isDraft,isPrerelease --jq '.isDraft or .isPrerelease')" = false
gh release download "$tag" --repo "$repo" --pattern 'Relaydroid-macOS-*.zip' --dir "$assets"
for arch in arm64 amd64; do
  zip="$assets/Relaydroid-macOS-$arch.zip"
  ditto -x -k "$zip" "$assets/$arch"
  app="$assets/$arch/Relaydroid.app"
  if [[ "$arch" == arm64 ]]; then machine=arm64; else machine=x86_64; fi
  test "$(lipo -archs "$app/Contents/MacOS/Relaydroid")" = "$machine"
  test "$(plutil -extract CFBundleShortVersionString raw "$app/Contents/Info.plist")" = "${tag#v}"
  codesign --verify --deep --strict "$app"
  xcrun stapler validate "$app"
done

(cd "$assets" && shasum -a 256 Relaydroid-macOS-*.zip) > "$checksums"
