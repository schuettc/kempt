package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func CheckSymlinks(ctx *machine.Context, pkgs []*manifest.Package) []Finding {
	var out []Finding
	for _, pkg := range pkgs {
		for _, step := range pkg.Steps {
			sl, ok := step.(manifest.SymlinkStep)
			if !ok {
				continue
			}
			to := ctx.Expand(sl.To)
			fi, err := os.Lstat(to)
			if err != nil {
				continue // absent → plan will create it
			}
			want := filepath.Join(ctx.RepoDir, sl.From)
			if fi.Mode()&os.ModeSymlink == 0 {
				out = append(out, Finding{
					Check: "symlink", Package: pkg.Name, Severity: Warn,
					Detail:      fmt.Sprintf("%s is a real %s where a managed symlink is expected", to, kindOf(fi)),
					Remediation: "run `kempt apply` (it will back up and relink)",
				})
				continue
			}
			target, _ := os.Readlink(to)
			if _, err := os.Stat(to); err != nil {
				out = append(out, Finding{
					Check: "symlink", Package: pkg.Name, Severity: Error,
					Detail:      fmt.Sprintf("%s is a broken symlink → %s", to, target),
					Remediation: "run `kempt apply` to relink, or remove the dangling link",
				})
				continue
			}
			if target != want {
				out = append(out, Finding{
					Check: "symlink", Package: pkg.Name, Severity: Warn,
					Detail:      fmt.Sprintf("%s points to %s, expected %s", to, target, want),
					Remediation: "run `kempt apply` to repoint it",
				})
			}
		}
	}
	return out
}

func kindOf(fi os.FileInfo) string {
	if fi.IsDir() {
		return "directory"
	}
	return "file"
}
