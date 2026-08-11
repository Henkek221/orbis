#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist/mappy"
VERSION="${1:-dev}"

rm -rf "$DIST"
mkdir -p "$DIST/work" "$DIST/releases"

build_go() {
  local goos="$1"
  local goarch="$2"
  local output="$3"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -mod=vendor -trimpath -ldflags="-s -w" -o "$output" ./cmd/export-ui
}

cd "$ROOT"

build_go darwin arm64 "$DIST/work/mappy-darwin-arm64"
build_go darwin amd64 "$DIST/work/mappy-darwin-amd64"
mkdir -p "$DIST/work/mac/Mappy.app/Contents/MacOS"
if command -v lipo >/dev/null 2>&1; then
  lipo -create "$DIST/work/mappy-darwin-arm64" "$DIST/work/mappy-darwin-amd64" \
    -output "$DIST/work/mac/Mappy.app/Contents/MacOS/Mappy"
else
  cp "$DIST/work/mappy-darwin-arm64" "$DIST/work/mac/Mappy.app/Contents/MacOS/Mappy"
fi
chmod +x "$DIST/work/mac/Mappy.app/Contents/MacOS/Mappy"
cat > "$DIST/work/mac/Mappy.app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>de</string>
  <key>CFBundleDisplayName</key>
  <string>Mappy</string>
  <key>CFBundleExecutable</key>
  <string>Mappy</string>
  <key>CFBundleIdentifier</key>
  <string>local.mappy.app</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>Mappy</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>0.1</string>
  <key>CFBundleVersion</key>
  <string>1</string>
  <key>LSMinimumSystemVersion</key>
  <string>11.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST
if command -v codesign >/dev/null 2>&1; then
  xattr -cr "$DIST/work/mac/Mappy.app"
  codesign --force --deep --sign - "$DIST/work/mac/Mappy.app"
fi
cp packaging/HIER-STARTEN-macOS.txt "$DIST/work/mac/HIER STARTEN.txt"
cp DISTRIBUTION.md "$DIST/work/mac/README.md"
cp CREDITS.md "$DIST/work/mac/CREDITS.md"
(cd "$DIST/work/mac" && zip -qry "$DIST/releases/mappy-${VERSION}-mac.zip" .)

mkdir -p "$DIST/work/windows"
build_go windows amd64 "$DIST/work/windows/Mappy.exe"
cp "packaging/MAPPY STARTEN.cmd" "$DIST/work/windows/MAPPY STARTEN.cmd"
cp packaging/HIER-STARTEN-Windows.txt "$DIST/work/windows/HIER STARTEN.txt"
cp config.example.json "$DIST/work/windows/config.example.json"
cp DISTRIBUTION.md "$DIST/work/windows/README.md"
cp CREDITS.md "$DIST/work/windows/CREDITS.md"
(cd "$DIST/work/windows" && zip -qry "$DIST/releases/mappy-${VERSION}-windows.zip" .)

mkdir -p "$DIST/work/linux-x64" "$DIST/work/linux-arm64"
build_go linux amd64 "$DIST/work/linux-x64/Mappy"
build_go linux arm64 "$DIST/work/linux-arm64/Mappy"
chmod +x "$DIST/work/linux-x64/Mappy" "$DIST/work/linux-arm64/Mappy"
for folder in linux-x64 linux-arm64; do
  cp "packaging/MAPPY STARTEN.sh" "$DIST/work/$folder/MAPPY STARTEN.sh"
  chmod +x "$DIST/work/$folder/MAPPY STARTEN.sh"
  cp packaging/HIER-STARTEN-Linux.txt "$DIST/work/$folder/HIER STARTEN.txt"
  cp config.example.json "$DIST/work/$folder/config.example.json"
  cp DISTRIBUTION.md "$DIST/work/$folder/README.md"
  cp CREDITS.md "$DIST/work/$folder/CREDITS.md"
  (cd "$DIST/work/$folder" && zip -qry "$DIST/releases/mappy-${VERSION}-${folder}.zip" .)
done

printf 'Built Mappy releases in %s\n' "$DIST/releases"
