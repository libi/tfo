// Package tfo provides embedded frontend assets at the module root level.
// This file exists here because Go's //go:embed can only reference files
// in the same directory or subdirectories of the source file.
package tfo

import "embed"

// FrontendAssets contains the Next.js static export (frontend/out/).
// Build with: cd frontend && npm run build
//
//go:embed all:frontend/out
var FrontendAssets embed.FS
