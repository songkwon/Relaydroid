#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."
dist="${1:-$PWD/build}"
mkdir -p "$dist"
dist="$(cd "$dist" && pwd)"
app="$dist/Relay.app"
build="$(mktemp -d)"
trap 'rm -rf "$build"' EXIT
stage="$build/Relay.app"
arch="$(go env GOARCH)"
export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-14.0}"
export CGO_ENABLED=1
export CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
export CGO_LDFLAGS="${CGO_LDFLAGS:--O2 -g} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
version="${RELAY_VERSION:-${GITHUB_REF_NAME:-0.2.0}}"
version="${version#v}"
if [[ "$version" =~ ^([0-9]+\.[0-9]+\.[0-9]+)([-+].*)?$ ]]; then
  version="${BASH_REMATCH[1]}"
else
  version=0.2.0
fi
build_number="${GITHUB_RUN_NUMBER:-2}"

if [ -d "$PWD/build/native/libmtp/lib/pkgconfig" ]; then
  export PKG_CONFIG_PATH="${PKG_CONFIG_PATH:+$PKG_CONFIG_PATH:}$PWD/build/native/libmtp/lib/pkgconfig:$PWD/build/native/libusb/lib/pkgconfig"
fi

mtp_lib="$(pkg-config --variable=libdir libmtp)"
usb_lib="$(pkg-config --variable=libdir libusb-1.0)"
mtp_source="$(find -L "$mtp_lib" -maxdepth 1 -name 'libmtp.9.dylib' -print -quit)"
usb_source="$(find -L "$usb_lib" -maxdepth 1 -name 'libusb-1.0.0.dylib' -print -quit)"
[ -f "$mtp_source" ] && [ -f "$usb_source" ]

npm --prefix frontend ci --no-audit --no-fund
npm --prefix frontend run build
go test -tags mtp -ldflags "-extldflags=-mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET" ./...
mkdir -p "$stage/Contents/"{MacOS,Frameworks,Resources/Licenses} "$build/Relay.iconset"
go build -trimpath -tags mtp -ldflags "-s -w -extldflags=-mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET" -o "$stage/Contents/MacOS/Relay" .
swift scripts/icon.swift "$build/icon.png"
for n in 16 32 128 256 512; do
  sips -z "$n" "$n" "$build/icon.png" --out "$build/Relay.iconset/icon_${n}x${n}.png" >/dev/null
  sips -z "$((n * 2))" "$((n * 2))" "$build/icon.png" --out "$build/Relay.iconset/icon_${n}x${n}@2x.png" >/dev/null
done
iconutil -c icns "$build/Relay.iconset" -o "$stage/Contents/Resources/Relay.icns"

mtp="$stage/Contents/Frameworks/libmtp.9.dylib"
usb="$stage/Contents/Frameworks/libusb-1.0.0.dylib"
binary="$stage/Contents/MacOS/Relay"
cp -L "$mtp_source" "$mtp"
cp -L "$usb_source" "$usb"
chmod u+w "$mtp" "$usb" "$binary"
mtp_dependency="$(otool -L "$binary" | awk '/libmtp.*\.dylib/{print $1; exit}')"
usb_binary_dependency="$(otool -L "$binary" | awk '/libusb.*\.dylib/{print $1; exit}')"
usb_dependency="$(otool -L "$mtp" | awk '/libusb.*\.dylib/{print $1; exit}')"
install_name_tool -change "$mtp_dependency" '@executable_path/../Frameworks/libmtp.9.dylib' \
  -change "$usb_binary_dependency" '@executable_path/../Frameworks/libusb-1.0.0.dylib' "$binary"
install_name_tool -id '@rpath/libmtp.9.dylib' -change "$usb_dependency" '@loader_path/libusb-1.0.0.dylib' "$mtp"
install_name_tool -id '@rpath/libusb-1.0.0.dylib' "$usb"

cat > "$stage/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleExecutable</key><string>Relay</string>
  <key>CFBundleIdentifier</key><string>app.relaytransfer.desktop</string>
  <key>CFBundleName</key><string>Relay</string>
  <key>CFBundleDisplayName</key><string>Relay · 渡线</string>
  <key>CFBundleIconFile</key><string>Relay.icns</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$version</string>
  <key>CFBundleVersion</key><string>$build_number</string>
  <key>LSMinimumSystemVersion</key><string>$MACOSX_DEPLOYMENT_TARGET</string>
  <key>LSApplicationCategoryType</key><string>public.app-category.utilities</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSLocalNetworkUsageDescription</key><string>Relay 在局域网中连接你配对的 Android 设备。</string>
</dict></plist>
PLIST

licenses="$stage/Contents/Resources/Licenses"
cp frontend/src/rare-ui/LICENSE "$licenses/Rare-UI.txt"
cp scripts/native-notices.txt "$licenses/Native-Dependencies.txt"
mtp_license="${RELAY_LIBMTP_LICENSE:-$(dirname "$mtp_lib")/COPYING}"
usb_license="${RELAY_LIBUSB_LICENSE:-$(dirname "$usb_lib")/COPYING}"
cp "$mtp_license" "$licenses/libmtp.txt"
cp "$usb_license" "$licenses/libusb.txt"
go list -tags mtp -deps -f '{{with .Module}}{{if and (not .Main) .Dir}}{{.Path}} {{.Dir}}{{end}}{{end}}' . |
  sort -u | while read -r module dir; do
    [ -n "$dir" ] || continue
    filename="${module//\//_}"
    if [ -f "$dir/LICENSE" ]; then cp "$dir/LICENSE" "$licenses/$filename.txt"; fi
    if [ -f "$dir/NOTICE" ]; then cp "$dir/NOTICE" "$licenses/$filename-NOTICE.txt"; fi
  done
cp "$(go env GOROOT)/LICENSE" "$licenses/Go.txt"
npm --prefix frontend ls --omit=dev --all --parseable |
  while IFS= read -r module; do
    [ "$module" = "$PWD/frontend" ] && continue
    for notice in "$module"/{LICENSE,LICENSE.md,LICENSE.txt,LICENCE,NOTICE}; do
      if [ -f "$notice" ]; then cp "$notice" "$licenses/npm-$(basename "$module")-$(basename "$notice").txt"; fi
    done
  done

plutil -lint "$stage/Contents/Info.plist"
identity="${RELAY_CODESIGN_IDENTITY:--}"
codesign --force --sign "$identity" "$usb"
codesign --force --sign "$identity" "$mtp"
codesign --force --sign "$identity" "$stage"
codesign --verify --deep --strict "$stage"
mkdir "$build/release"
cp README.md "$build/release/README.md"
ditto "$stage" "$build/release/Relay.app"
ditto -c -k --sequesterRsrc "$build/release" "$dist/Relay-macOS-$arch.zip"
if [ -e "$app" ]; then mv "$app" "$build/previous.app"; fi
if ! mv "$build/release/Relay.app" "$app"; then
  if [ -e "$build/previous.app" ]; then mv "$build/previous.app" "$app"; fi
  exit 1
fi
printf '%s\n' "$dist/Relay-macOS-$arch.zip"
