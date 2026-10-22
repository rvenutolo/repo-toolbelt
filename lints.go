// Package lintsmanifest embeds the lint manifest, lints.toml, at the repo root.
// The embed directive cannot reach a parent directory, so the root package
// exposes the bytes for internal/manifest to parse.
package lintsmanifest

import _ "embed" // required for the go:embed directive below

// TOML is the raw content of lints.toml.
//
//go:embed lints.toml
var TOML []byte
