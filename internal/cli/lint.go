package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/schuettc/kempt/internal/manifest"
)

func init() {
	Register(Command{Name: "lint", Summary: "validate a kempt.toml", Run: runLint})
}

func runLint(args []string, out, errw io.Writer) error {
	path := "kempt.toml"
	if len(args) > 1 {
		return UsageError{Msg: "usage: kempt lint [path]"}
	}
	if len(args) == 1 {
		path = args[0]
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return UsageError{Msg: fmt.Sprintf("cannot read %s: %v", path, err)}
	}
	m, findings := manifest.Parse(src)
	if m != nil {
		findings = append(findings, manifest.Validate(m)...)
	}
	for _, f := range findings {
		fmt.Fprintf(out, "%s: %s: %s\n", path, f.Path, f.Msg)
	}
	if len(findings) > 0 {
		return fmt.Errorf("%d finding(s)", len(findings))
	}
	return nil
}
