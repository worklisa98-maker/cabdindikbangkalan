package assets

import "embed"

// FS contains static assets bundled into the backend binary.
//
//go:embed *
var FS embed.FS
