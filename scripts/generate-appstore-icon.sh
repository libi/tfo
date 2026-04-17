#!/bin/bash
# Generate App Store icon assets from a source 1024x1024 PNG
# Usage: ./scripts/generate-appstore-icon.sh [source.png]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="${1:-$ROOT/cmd/desktop/icons/icon.png}"
ICONSET="$ROOT/cmd/desktop/macos/TFOApp/Resources/AppIcon.iconset"

if [[ ! -f "$SOURCE" ]]; then
  echo "❌ Source icon not found: $SOURCE"
  echo "   Provide a 1024x1024 PNG as argument or place it at cmd/desktop/icons/icon.png"
  exit 1
fi

mkdir -p "$ICONSET"

# All sizes required for macOS App Store
declare -a SIZES=(16 32 64 128 256 512 1024)

for size in "${SIZES[@]}"; do
  sips -z "$size" "$size" "$SOURCE" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
  echo "  ✓ ${size}x${size}"
done

# @2x variants
for size in 16 32 128 256 512; do
  actual=$((size * 2))
  cp "$ICONSET/icon_${actual}x${actual}.png" "$ICONSET/icon_${size}x${size}@2x.png" 2>/dev/null || \
    sips -z "$actual" "$actual" "$SOURCE" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
  echo "  ✓ ${size}x${size}@2x"
done

# Generate .icns from iconset
iconutil -c icns "$ICONSET" -o "$ROOT/cmd/desktop/macos/TFOApp/Resources/AppIcon.icns"
echo "  ✓ AppIcon.icns"

# Copy 1024x1024 as App Store marketing icon
cp "$ICONSET/icon_1024x1024.png" "$ROOT/cmd/desktop/macos/TFOApp/Resources/AppStoreIcon.png"
echo "  ✓ AppStoreIcon.png (1024x1024 marketing icon)"

echo "Done! Icons generated in Resources/"
