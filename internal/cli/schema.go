package cli

import (
	"io"

	"github.com/schuettc/kempt/internal/schema"
)

func init() {
	Register(Command{Name: "schema", Summary: "print the JSON Schema for kempt.toml", Run: runSchema})
}

func runSchema(args []string, out, errw io.Writer) error {
	if len(args) > 0 {
		return UsageError{Msg: "usage: kempt schema"}
	}
	out.Write(schema.JSON())
	io.WriteString(out, "\n")
	return nil
}
