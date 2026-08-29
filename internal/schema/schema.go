// Package schema provides the JSON Schema (draft 2020-12) describing kempt.toml.
package schema

import _ "embed"

//go:embed schema.json
var schemaJSON []byte

// JSON returns the embedded JSON Schema bytes for kempt.toml.
func JSON() []byte { return schemaJSON }
