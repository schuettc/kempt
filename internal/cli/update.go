package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/schuettc/kempt/internal/engine"
	_ "github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/gitrepo"
	"github.com/schuettc/kempt/internal/manifest"
	tools "github.com/schuettc/tools-common"
)

// selfUpdate is a seam so tests can stub the binary-replace step (which
// otherwise reaches the network via the /dl download contract).
var selfUpdate = func(app *tools.App, out, errw io.Writer) (bool, string, error) {
	return app.SelfUpdate(out, errw)
}

func runUpdate(app *tools.App, args []string, out, errw io.Writer) error {
	fset := flag.NewFlagSet("update", flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	if err := fset.Parse(args); err != nil {
		return UsageError{Msg: err.Error()}
	}

	st, existed, err := loadState()
	if err != nil {
		return err
	}
	if !existed {
		return UsageError{Msg: "no saved selection; run kempt init first"}
	}

	ctx, err := newContext(st.RepoDir)
	if err != nil {
		return err
	}

	// 1. Refresh the config tree. A tarball-sourced config is re-fetched and
	// re-extracted; a git repo is pulled (a real conflict must surface).
	if st.RepoKind == "tarball" {
		if st.RepoURL == "" {
			return UsageError{Msg: "tarball-sourced config has no saved URL to re-fetch"}
		}
		if err := fetchTarball(st.RepoURL, st.RepoDir); err != nil {
			return fmt.Errorf("re-fetch %s: %w", st.RepoURL, err)
		}
	} else if err := gitrepo.Pull(ctx.Runner, st.RepoDir); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	// 2. Self-update the binary via the /dl download contract. A non-writable
	// exe dir is a soft failure: we still converge config. Other errors abort.
	updated, newVer, uerr := selfUpdate(app, out, errw)
	if uerr != nil {
		if isPermissionErr(uerr) {
			fmt.Fprintf(out, "binary self-update skipped: %v\n", uerr)
		} else {
			return uerr
		}
	} else if updated {
		fmt.Fprintf(out, "kempt updated to %s\n", newVer)
	}

	// 3. Converge config from the freshly-pulled repo.
	manifestPath := filepath.Join(st.RepoDir, "kempt.toml")
	src, err := os.ReadFile(manifestPath)
	if err != nil {
		return UsageError{Msg: fmt.Sprintf("cannot read %s: %v", manifestPath, err)}
	}
	m, findings := manifest.Parse(src)
	if m != nil {
		findings = append(findings, manifest.Validate(m)...)
	}
	if len(findings) > 0 {
		for _, f := range findings {
			fmt.Fprintf(errw, "%s: %s: %s\n", manifestPath, f.Path, f.Msg)
		}
		return fmt.Errorf("manifest has findings; run kempt lint")
	}

	selected, err := engine.Select(m, "", st.Packages)
	if err != nil {
		return UsageError{Msg: err.Error()}
	}
	plan, err := engine.BuildPlan(ctx, selected)
	if err != nil {
		return err
	}
	engine.Render(plan, out)

	applied, failed := executeAndVerify(ctx, plan, out)
	blocked := countBlocked(plan)
	fmt.Fprintf(out, "%d applied, %d failed\n", applied, failed)
	if blocked > 0 {
		fmt.Fprintf(out, "%d blocked (unresolved)\n", blocked)
	}
	if failed > 0 {
		return fmt.Errorf("%d step(s) failed", failed)
	}
	return nil
}

// isPermissionErr reports whether err is a permission/not-writable error on the
// exe directory, in which case self-update is skipped rather than fatal.
func isPermissionErr(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}
