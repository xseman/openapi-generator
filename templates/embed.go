// Package templates provides embedded template files for code generation.
package templates

import "embed"

// FS contains all embedded template files.
//
//go:embed typescript-fetch/*.mustache typescript-fetch/*.md dart-fetch/*.mustache
var FS embed.FS
