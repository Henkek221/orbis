#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist/orbis"
VERSION="${1:-dev}"

rm -rf "$DIST"
mkdir -p "$DIST/work" "$DIST/releases"

build_go() {
  local goos="$1"
  local goarch="$2"
  local output="$3"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -mod=vendor -trimpath -ldflags="-s -w -X main.version=$VERSION" -o "$output" ./cmd/orbis
}

cd "$ROOT"

build_go darwin arm64 "$DIST/work/orbis-darwin-arm64"
build_go darwin amd64 "$DIST/work/orbis-darwin-amd64"
mkdir -p "$DIST/work/mac/Orbis.app/Contents/MacOS"
if command -v lipo >/dev/null 2>&1; then
  lipo -create "$DIST/work/orbis-darwin-arm64" "$DIST/work/orbis-darwin-amd64" \
    -output "$DIST/work/mac/Orbis.app/Contents/MacOS/Orbis"
else
  cp "$DIST/work/orbis-darwin-arm64" "$DIST/work/mac/Orbis.app/Contents/MacOS/Orbis"
fi
chmod +x "$DIST/work/mac/Orbis.app/Contents/MacOS/Orbis"
cat > "$DIST/work/mac/Orbis.app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>de</string>
  <key>CFBundleDisplayName</key>
  <string>Orbis</string>
  <key>CFBundleExecutable</key>
  <string>Orbis</string>
  <key>CFBundleIdentifier</key>
  <string>local.orbis.app</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>Orbis</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>0.2</string>
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
  xattr -cr "$DIST/work/mac/Orbis.app"
  codesign --force --deep --sign - "$DIST/work/mac/Orbis.app"
fi
cp packaging/HIER-STARTEN-macOS.txt "$DIST/work/mac/HIER STARTEN.txt"
cp DISTRIBUTION.md "$DIST/work/mac/README.md"
cp CREDITS.md "$DIST/work/mac/CREDITS.md"
(cd "$DIST/work/mac" && zip -qry "$DIST/releases/orbis-${VERSION}-mac.zip" .)

mkdir -p "$DIST/work/windows"
build_go windows amd64 "$DIST/work/windows/Orbis.exe"
cp "packaging/ORBIS STARTEN.cmd" "$DIST/work/windows/ORBIS STARTEN.cmd"
cp packaging/HIER-STARTEN-Windows.txt "$DIST/work/windows/HIER STARTEN.txt"
cp config.example.json "$DIST/work/windows/config.example.json"
cp DISTRIBUTION.md "$DIST/work/windows/README.md"
cp CREDITS.md "$DIST/work/windows/CREDITS.md"
(cd "$DIST/work/windows" && zip -qry "$DIST/releases/orbis-${VERSION}-windows.zip" .)

mkdir -p "$DIST/work/linux-x64" "$DIST/work/linux-arm64"
build_go linux amd64 "$DIST/work/linux-x64/Orbis"
build_go linux arm64 "$DIST/work/linux-arm64/Orbis"
chmod +x "$DIST/work/linux-x64/Orbis" "$DIST/work/linux-arm64/Orbis"
for folder in linux-x64 linux-arm64; do
  cp "packaging/ORBIS STARTEN.sh" "$DIST/work/$folder/ORBIS STARTEN.sh"
  chmod +x "$DIST/work/$folder/ORBIS STARTEN.sh"
  cp packaging/HIER-STARTEN-Linux.txt "$DIST/work/$folder/HIER STARTEN.txt"
  cp config.example.json "$DIST/work/$folder/config.example.json"
  cp DISTRIBUTION.md "$DIST/work/$folder/README.md"
  cp CREDITS.md "$DIST/work/$folder/CREDITS.md"
  (cd "$DIST/work/$folder" && zip -qry "$DIST/releases/orbis-${VERSION}-${folder}.zip" .)
done

printf 'Built Orbis releases in %s\n' "$DIST/releases"
