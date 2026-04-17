#!/bin/bash
# TFO build script
# Usage: ./scripts/build.sh [cli|desktop|all] [--os <os>] [--arch <arch>]
#
# Examples:
#   ./scripts/build.sh cli                        # build CLI for all platforms
#   ./scripts/build.sh cli --os darwin             # build CLI for macOS (all archs)
#   ./scripts/build.sh cli --os linux --arch amd64 # build CLI for linux/amd64 only
#   ./scripts/build.sh desktop --os windows        # build desktop for Windows only
#   ./scripts/build.sh desktop --os darwin          # build desktop macOS .app bundle
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/dist"
SWIFT_APP="$ROOT/cmd/desktop/macos/TFOApp"
BUNDLE_ID="${TFO_BUNDLE_ID:-com.libi.tfo}"
SIGN_IDENTITY="${TFO_SIGN_IDENTITY:--}"  # '-' = ad-hoc; set to 'Developer ID Application: ...' for release
INSTALLER_IDENTITY="${TFO_INSTALLER_IDENTITY:-}"  # '3rd Party Mac Developer Installer: ...' for App Store pkg
ENTITLEMENTS="$ROOT/cmd/desktop/macos/TFOApp/TFOApp.entitlements"
VERSION="${TFO_VERSION:-1.0.0}"
BUILD_NUMBER="${TFO_BUILD_NUMBER:-$(date +%Y%m%d%H%M)}"
# Notarization credentials (required for notarize step)
APPLE_ID="${TFO_APPLE_ID:-}"                     # Apple ID email
APPLE_TEAM_ID="${TFO_APPLE_TEAM_ID:-}"           # Developer Team ID
APPLE_APP_PASSWORD="${TFO_APPLE_APP_PASSWORD:-}" # App-specific password
LDFLAGS="-s -w"

# --- parse args ---
CMD="${1:-all}"; shift || true
TARGET_OS="" ; TARGET_ARCH=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --os)   TARGET_OS="$2";   shift 2 ;;
    --arch) TARGET_ARCH="$2"; shift 2 ;;
    *)      echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# return 0 if the given os/arch should be built
should_build() { # $1=os $2=arch
  [[ -z "$TARGET_OS"   || "$TARGET_OS"   = "$1" ]] &&
  [[ -z "$TARGET_ARCH" || "$TARGET_ARCH" = "$2" ]]
}

build_frontend() {
  [[ -d "$ROOT/frontend/out" && "${SKIP_WEB:-0}" = "1" ]] && return
  (cd "$ROOT/frontend" && npm install --silent && npm run build)
}

build_go() { # $1=os $2=arch $3=pkg $4=output
  CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -trimpath -ldflags "$LDFLAGS" -o "$4" "$3"
  echo "  ✓ $4"
}

build_cli() {
  echo "=== CLI ==="
  build_frontend
  for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
    os="${target%/*}"; arch="${target#*/}"
    should_build "$os" "$arch" || continue
    ext=""; [[ "$os" = "windows" ]] && ext=".exe"
    build_go "$os" "$arch" ./cmd/tfo "$DIST/cli/${os}_${arch}/tfo-${VERSION}-${os}-${arch}${ext}"
  done
}

build_desktop_windows() {
  # Generate Windows resources (icon, manifest) via go-winres
  if command -v go-winres &>/dev/null; then
    (cd "$ROOT/cmd/desktop" && go-winres make --in winres/winres.json)
    echo "  ✓ winres generated"
  else
    echo "  ⚠ go-winres not found, install with: go install github.com/tc-hib/go-winres@latest"
  fi

  for arch in amd64; do
    should_build windows "$arch" || continue
    # -H windowsgui hides the console window on launch
    CGO_ENABLED=0 GOOS=windows GOARCH="$arch" go build -trimpath \
      -tags production \
      -ldflags "$LDFLAGS -H windowsgui" \
      -o "$DIST/desktop/windows_${arch}/TFO-${VERSION}-windows-${arch}-desktop.exe" ./cmd/desktop
    echo "  ✓ $DIST/desktop/windows_${arch}/TFO-${VERSION}-windows-${arch}-desktop.exe"
  done
}

build_desktop_darwin() {
  local archs=()
  should_build darwin arm64 && archs+=(arm64)
  should_build darwin amd64 && archs+=(amd64)
  [[ ${#archs[@]} -eq 0 ]] && return

  for arch in "${archs[@]}"; do
    local swift_arch; [[ "$arch" = "amd64" ]] && swift_arch="x86_64" || swift_arch="$arch"
    local app="$DIST/desktop/darwin_${arch}/TFO.app/Contents"
    mkdir -p "$app/MacOS" "$app/Resources"

    # Go binary
    build_go darwin "$arch" ./cmd/desktop "$app/Resources/tfo-desktop"

    # Swift launcher (single arch)
    (cd "$SWIFT_APP" && swift package clean >/dev/null 2>&1; swift build -c release --arch "$swift_arch" 2>&1 | tail -1)
    cp "$SWIFT_APP/.build/${swift_arch}-apple-macosx/release/TFOApp" "$app/MacOS/TFOApp"

    # Info.plist & resources
    sed -e "s/\$(PRODUCT_BUNDLE_IDENTIFIER)/$BUNDLE_ID/g" \
        -e "s/<string>1.0.0<\/string>/<string>$VERSION<\/string>/" \
        -e "s/<key>CFBundleVersion<\/key>\n.*<string>.*<\/string>/<key>CFBundleVersion<\/key>\n    <string>$BUILD_NUMBER<\/string>/" \
        "$SWIFT_APP/Info.plist" > "$app/Info.plist"
    # Patch CFBundleVersion separately for reliable replacement
    sed -i '' "/<key>CFBundleVersion<\/key>/{n;s/<string>.*<\/string>/<string>$BUILD_NUMBER<\/string>/;}" "$app/Info.plist"
    for f in "$SWIFT_APP/Resources/"*; do [[ -f "$f" ]] && cp "$f" "$app/Resources/"; done 2>/dev/null || true

    # Select entitlements based on distribution target:
    # - App Store (sandbox required): TFOApp.entitlements
    # - Developer ID / ad-hoc (no sandbox): TFOApp-devid.entitlements
    local app_entitlements="$ENTITLEMENTS"
    if [[ -z "$INSTALLER_IDENTITY" ]]; then
      app_entitlements="$ROOT/cmd/desktop/macos/TFOApp/TFOApp-devid.entitlements"
    fi

    # Code sign the Go helper binary with direct entitlements (no sandbox).
    # The helper inherits the sandbox from the Swift launcher when run inside
    # the .app bundle, so it must NOT carry sandbox entitlements itself —
    # otherwise it crashes with SIGILL when the codesign attributes conflict.
    local helper_entitlements="$ROOT/cmd/desktop/macos/TFOApp/TFOApp-direct.entitlements"
    codesign --force --options runtime --sign "$SIGN_IDENTITY" \
      --entitlements "$helper_entitlements" \
      "$app/Resources/tfo-desktop"

    # Code sign the .app bundle with Hardened Runtime
    codesign --force --deep --options runtime --sign "$SIGN_IDENTITY" \
      --entitlements "$app_entitlements" \
      "$DIST/desktop/darwin_${arch}/TFO.app"
    echo "  ✓ $DIST/desktop/darwin_${arch}/TFO.app (signed: $SIGN_IDENTITY)"

    # Build .pkg for App Store submission if installer identity is set
    if [[ -n "$INSTALLER_IDENTITY" ]]; then
      local pkg="$DIST/desktop/darwin_${arch}/TFO.pkg"
      productbuild --component "$DIST/desktop/darwin_${arch}/TFO.app" /Applications \
        --sign "$INSTALLER_IDENTITY" "$pkg"
      echo "  ✓ $pkg (App Store package)"
    fi

    # --- Create DMG ---
    create_dmg "$DIST/desktop/darwin_${arch}" "$arch"
  done
}

# create_dmg builds a .dmg file with a drag-to-Applications layout.
# $1 = directory containing TFO.app, $2 = arch label
create_dmg() {
  local dir="$1" arch="$2"
  local dmg_name="TFO-${VERSION}-macOS-${arch}.dmg"
  local dmg_path="$DIST/$dmg_name"
  local tmp_dmg="$DIST/.tmp_${arch}.dmg"
  local vol_name="TFO ${VERSION}"
  local staging="$DIST/.dmg_staging_${arch}"

  echo "  ● Creating DMG: $dmg_name"

  # Prepare staging directory
  rm -rf "$staging"
  mkdir -p "$staging"
  cp -a "$dir/TFO.app" "$staging/"
  ln -s /Applications "$staging/Applications"

  # Create temporary read-write DMG
  rm -f "$tmp_dmg" "$dmg_path"
  hdiutil create -srcfolder "$staging" -volname "$vol_name" \
    -fs HFS+ -fsargs "-c c=64,a=16,e=16" \
    -format UDRW -size 200m "$tmp_dmg" >/dev/null

  # Mount and customise appearance
  local device
  device=$(hdiutil attach -readwrite -noverify -noautoopen "$tmp_dmg" \
    | grep -E '^/dev/' | head -1 | awk '{print $1}')

  # AppleScript to set icon positions and window size
  osascript <<EOF
tell application "Finder"
  tell disk "$vol_name"
    open
    set current view of container window to icon view
    set toolbar visible of container window to false
    set statusbar visible of container window to false
    set bounds of container window to {100, 100, 640, 400}
    set theViewOptions to icon view options of container window
    set arrangement of theViewOptions to not arranged
    set icon size of theViewOptions to 80
    set position of item "TFO.app" of container window to {140, 150}
    set position of item "Applications" of container window to {400, 150}
    close
    open
    update without registering applications
    delay 1
    close
  end tell
end tell
EOF

  # Set volume icon if available
  local vol_icon="$ROOT/cmd/desktop/icons/tfo.icns"
  if [[ -f "$vol_icon" ]]; then
    cp "$vol_icon" "/Volumes/$vol_name/.VolumeIcon.icns"
    SetFile -c icnC "/Volumes/$vol_name/.VolumeIcon.icns" 2>/dev/null || true
    SetFile -a C "/Volumes/$vol_name" 2>/dev/null || true
  fi

  # Unmount, convert to compressed read-only DMG
  hdiutil detach "$device" -quiet
  hdiutil convert "$tmp_dmg" -format UDZO -imagekey zlib-level=9 -o "$dmg_path" >/dev/null

  rm -f "$tmp_dmg"
  rm -rf "$staging"

  # Sign the DMG itself
  if [[ "$SIGN_IDENTITY" != "-" ]]; then
    codesign --force --sign "$SIGN_IDENTITY" "$dmg_path"
    echo "  ✓ DMG signed"
  fi

  # Notarize if credentials are available
  notarize_dmg "$dmg_path"

  echo "  ✓ $dmg_path"
}

# notarize_dmg submits the DMG to Apple notary service and staples the ticket.
notarize_dmg() {
  local dmg="$1"
  if [[ -z "$APPLE_ID" || -z "$APPLE_TEAM_ID" || -z "$APPLE_APP_PASSWORD" ]]; then
    echo "  ⚠ Skipping notarization (set TFO_APPLE_ID, TFO_APPLE_TEAM_ID, TFO_APPLE_APP_PASSWORD)"
    return
  fi
  if [[ "$SIGN_IDENTITY" = "-" ]]; then
    echo "  ⚠ Skipping notarization (ad-hoc signing)"
    return
  fi

  echo "  ● Submitting to Apple notary service..."
  xcrun notarytool submit "$dmg" \
    --apple-id "$APPLE_ID" \
    --team-id "$APPLE_TEAM_ID" \
    --password "$APPLE_APP_PASSWORD" \
    --wait

  echo "  ● Stapling notarization ticket..."
  xcrun stapler staple "$dmg"
  echo "  ✓ Notarization complete"
}

build_desktop() {
  echo "=== Desktop ==="
  build_frontend
  build_desktop_windows
  build_desktop_darwin
}

cd "$ROOT"; mkdir -p "$DIST"
case "$CMD" in
  cli)     build_cli ;;
  desktop) build_desktop ;;
  all)     build_cli; build_desktop ;;
  *)       echo "Usage: $0 [cli|desktop|all] [--os <os>] [--arch <arch>]"; exit 1 ;;
esac
echo "Done → $DIST/"
