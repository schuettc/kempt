package machine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/schuettc/kempt/internal/release"
	"github.com/schuettc/kempt/internal/run"
)

// Context holds the machine-level state for kempt operations.
type Context struct {
	Home     string
	RepoDir  string
	OS       string // runtime.GOOS at construction; overridable in tests
	Arch     string // runtime.GOARCH at construction; overridable in tests
	Runner   run.Runner
	Releases release.Releases // wired in Task 7; nil until then
}

// New constructs a Context for the given repository directory using the
// provided Runner. Home is resolved from os.UserHomeDir; OS and Arch are
// taken from runtime.GOOS and runtime.GOARCH.
func New(repoDir string, r run.Runner) (*Context, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Context{
		Home:    home,
		RepoDir: repoDir,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Runner:  r,
	}, nil
}

// Expand resolves a path relative to the Context:
//   - "~"        → Home
//   - "~/x"      → Home/x
//   - absolute   → unchanged
//   - anything else → filepath.Join(RepoDir, p)
func (c *Context) Expand(p string) string {
	if p == "~" {
		return c.Home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(c.Home, p[2:])
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.RepoDir, p)
}
