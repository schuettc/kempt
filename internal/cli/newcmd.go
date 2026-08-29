package cli

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:embed templates/kempt.toml
var kemptTemplate []byte

//go:embed templates/README.md
var readmeTemplate []byte

//go:embed templates/kempt-lint.yml
var workflowTemplate []byte

func init() {
	Register(Command{
		Name:    "new",
		Summary: "scaffold a new kempt config repo",
		Run:     runNew,
	})
}

func runNew(args []string, out, errw io.Writer) error {
	if len(args) > 1 {
		return UsageError{Msg: "usage: kempt new [dir]"}
	}
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}

	manifestPath := filepath.Join(dir, "kempt.toml")
	if _, err := os.Stat(manifestPath); err == nil {
		return UsageError{Msg: fmt.Sprintf("%s already exists; refusing to overwrite", manifestPath)}
	}

	workflowPath := filepath.Join(dir, ".github", "workflows", "kempt-lint.yml")
	readmePath := filepath.Join(dir, "README.md")

	files := []struct {
		path    string
		content []byte
	}{
		{manifestPath, kemptTemplate},
		{readmePath, readmeTemplate},
		{workflowPath, workflowTemplate},
	}

	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(f.path, f.content, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "created %s\n", f.path)
	}
	return nil
}
