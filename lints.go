// Package lintsmanifest embeds the lint manifest, lints.toml, and the formatter
// manifest, formatters.toml, both at the repo root. The embed directive cannot
// reach a parent directory, so the root package exposes the bytes for
// internal/manifest to parse.
package lintsmanifest

import _ "embed" // required for the go:embed directives below

// TOML is the raw content of lints.toml.
//
//go:embed lints.toml
var TOML []byte

// Formatters is the raw content of formatters.toml.
//
//go:embed formatters.toml
var Formatters []byte
