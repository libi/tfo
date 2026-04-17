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
SIGN_IDENTITY="${TFO_SIGN_IDENTITY:--}"  # '-' = ad-hoc; set to 'Developer ID Application: ...' or '3rd Party Mac Developer Application: ...' for release
INSTALLER_IDENTITY="${TFO_INSTALLER_IDENTITY:-}"  # '3rd Party Mac Developer Installer: ...' for App Store pkg
ENTITLEMENTS="$ROOT/cmd/desktop/macos/TFOApp/TFOApp.entitlements"
VERSION="${TFO_VERSION:-1.0.0}"
BUILD_NUMBER="${TFO_BUILD_NUMBER:-$(date +%Y%m%d%H%M)}"
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
    build_go "$os" "$arch" ./cmd/tfo "$DIST/cli/${os}_${arch}/tfo${ext}"
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
      -o "$DIST/desktop/windows_${arch}/tfo-desktop.exe" ./cmd/desktop
    echo "  ✓ $DIST/desktop/windows_${arch}/tfo-desktop.exe"
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

    # Code sign the Go helper binary first
    codesign --force --options runtime --sign "$SIGN_IDENTITY" \
      --entitlements "$ENTITLEMENTS" \
      "$app/Resources/tfo-desktop"

    # Code sign the .app bundle with Hardened Runtime
    codesign --force --deep --options runtime --sign "$SIGN_IDENTITY" \
      --entitlements "$ENTITLEMENTS" \
      "$DIST/desktop/darwin_${arch}/TFO.app"
    echo "  ✓ $DIST/desktop/darwin_${arch}/TFO.app (signed: $SIGN_IDENTITY)"

    # Build .pkg for App Store submission if installer identity is set
    if [[ -n "$INSTALLER_IDENTITY" ]]; then
      local pkg="$DIST/desktop/darwin_${arch}/TFO.pkg"
      productbuild --component "$DIST/desktop/darwin_${arch}/TFO.app" /Applications \
        --sign "$INSTALLER_IDENTITY" "$pkg"
      echo "  ✓ $pkg (App Store package)"
    fi
  done
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
