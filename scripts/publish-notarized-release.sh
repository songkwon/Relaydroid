#!/bin/bash
set -euo pipefail

if [[ $# != 3 || "$(uname -s)" != Darwin ]]; then
  echo "Usage on macOS: $0 vMAJOR.MINOR.PATCH PERSONAL_TEAM_ID NOTARYTOOL_PROFILE" >&2
  exit 2
fi

tag="$1"
team="$2"
profile="$3"
repo=songkwon/relaydroid

[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid release tag." >&2; exit 2; }
[[ "$team" =~ ^[A-Z0-9]{10}$ ]] || { echo "Invalid Apple Team ID." >&2; exit 2; }
identity="$(security find-identity -v -p codesigning |
  awk -v team="$team" 'index($0, "Developer ID Application:") && index($0, "(" team ")\042") { print $2; exit }')"
[[ "$identity" =~ ^[0-9A-Fa-f]{40}$ ]] || { echo "No Developer ID Application identity found for the selected team." >&2; exit 1; }

test "$(gh release view "$tag" --repo "$repo" --json isDraft --jq .isDraft)" = true
xcrun notarytool history --keychain-profile "$profile" >/dev/null

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
mkdir "$stage/assets"
gh release download "$tag" --repo "$repo" --dir "$stage/assets"
(cd "$stage/assets" && shasum -a 256 -c SHA256SUMS.txt)

for arch in arm64 amd64; do
  folder="$stage/$arch"
  zip="$stage/assets/Relaydroid-macOS-$arch.zip"
  ditto -x -k "$zip" "$folder"
  app="$folder/Relaydroid.app"
  if [[ "$arch" = arm64 ]]; then machine=arm64; else machine=x86_64; fi
  test "$(lipo -archs "$app/Contents/MacOS/Relaydroid")" = "$machine"
  test "$(plutil -extract CFBundleShortVersionString raw "$app/Contents/Info.plist")" = "${tag#v}"

  for library in "$app"/Contents/Frameworks/*.dylib; do
    codesign --force --timestamp --sign "$identity" "$library"
  done
  codesign --force --timestamp --options runtime --sign "$identity" "$app"
  codesign --verify --deep --strict "$app"
  codesign -dv --verbose=4 "$app" 2>&1 | grep -Fx "TeamIdentifier=$team"

  submission="$stage/submission-$arch.zip"
  ditto -c -k --sequesterRsrc --keepParent "$app" "$submission"
  xcrun notarytool submit "$submission" --keychain-profile "$profile" --wait
  xcrun stapler staple "$app"
  xcrun stapler validate "$app"
  rm -f "$zip"
  ditto -c -k --sequesterRsrc "$folder" "$zip"
done

cd "$stage/assets"
shasum -a 256 Relaydroid-*.zip libmtp-*.tar.gz libusb-*.tar.bz2 > SHA256SUMS.txt
test "$(gh release view "$tag" --repo "$repo" --json isDraft --jq .isDraft)" = true
gh release upload "$tag" --repo "$repo" --clobber Relaydroid-macOS-*.zip SHA256SUMS.txt
gh release edit "$tag" --repo "$repo" --draft=false --latest
