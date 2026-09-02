package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/schuettc/kempt/internal/engine"
	_ "github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/gitrepo"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/picker"
	"github.com/schuettc/kempt/internal/state"
)

func init() {
	Register(Command{
		Name:    "init",
		Summary: "fetch a config (git repo or tarball URL), choose a profile, and apply",
		Run:     runInit,
	})
}

// pickerRun is the interactive picker seam. It is a package var so init_test can
// inject a fixed Result without a TTY.
var pickerRun = picker.Run

func runInit(args []string, out, errw io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dirFlag := fs.String("dir", "", "clone target directory (default ~/.config/kempt/repo)")
	profileFlag := fs.String("profile", "", "profile to select (non-interactive)")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	manifestFlag := fs.String("manifest", "", "path to manifest within the repo (test override)")
	// Allow the positional repo-url to appear before or after flags by
	// re-parsing around each positional argument.
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return UsageError{Msg: err.Error()}
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}
	if len(positional) > 1 {
		return UsageError{Msg: "unexpected argument: " + positional[1]}
	}
	var url string
	if len(positional) > 0 {
		url = positional[0]
	}

	dir, err := resolveRepoDir(*dirFlag)
	if err != nil {
		return err
	}

	ctx, err := newContext(dir)
	if err != nil {
		return err
	}

	// Repo resolution. A tarball URL is fetched + extracted (no git); anything
	// else is a git repo cloned when the dir is not already the right checkout.
	repoKind := "git"
	if isTarballURL(url) {
		repoKind = "tarball"
		if err := fetchTarball(url, dir); err != nil {
			return fmt.Errorf("fetch %s: %w", url, err)
		}
	} else {
		origin, rerr := gitrepo.RemoteURL(ctx.Runner, dir)
		switch {
		case url != "":
			switch {
			case rerr != nil:
				// Not a repo (or absent): clone it.
				if err := gitrepo.Clone(ctx.Runner, url, dir); err != nil {
					return fmt.Errorf("clone %s: %w", url, err)
				}
			case origin == url:
				// Already the right checkout; reuse.
			default:
				return UsageError{Msg: fmt.Sprintf("%s already has origin %q, not %q", dir, origin, url)}
			}
		default:
			if rerr != nil {
				return UsageError{Msg: "provide a repo URL to clone"}
			}
			url = origin
		}
	}

	manifestPath := *manifestFlag
	if manifestPath == "" {
		manifestPath = filepath.Join(dir, "kempt.toml")
	}
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

	// Selection.
	var (
		chosenProfile  string
		chosenPackages []string
		selProfile     string
		selPackages    []string
	)
	switch {
	case *profileFlag != "":
		pr, ok := m.Profiles[*profileFlag]
		if !ok {
			return UsageError{Msg: fmt.Sprintf("unknown profile %q", *profileFlag)}
		}
		chosenProfile = *profileFlag
		chosenPackages = pr.Packages
		selProfile = *profileFlag
	case *yes:
		return UsageError{Msg: "-yes requires -profile for non-interactive init"}
	default:
		profiles, items := buildPickerInputs(m)
		res, err := pickerRun(profiles, items)
		if err != nil {
			return err
		}
		if !res.Confirmed {
			fmt.Fprintln(out, "init cancelled")
			return nil
		}
		chosenProfile = res.Profile
		chosenPackages = res.Packages
		selPackages = res.Packages
	}

	if err := saveState(&state.State{
		RepoDir:        dir,
		RepoURL:        url,
		RepoKind:       repoKind,
		Profile:        chosenProfile,
		Packages:       chosenPackages,
		AutoApplyFiles: false,
	}); err != nil {
		return err
	}

	selected, err := engine.Select(m, selProfile, selPackages)
	if err != nil {
		return UsageError{Msg: err.Error()}
	}
	plan, err := engine.BuildPlan(ctx, selected)
	if err != nil {
		return err
	}
	engine.Render(plan, out)

	changes := countChanges(plan)
	if changes == 0 {
		fmt.Fprintln(out, "nothing to do")
		return nil
	}

	if !*yes {
		fmt.Fprintf(out, "apply %d changes? [y/N] ", changes)
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			// proceed
		default:
			fmt.Fprintln(out, "aborted")
			return fmt.Errorf("aborted")
		}
	}

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

// resolveRepoDir returns the clone target: the -dir flag, or the default
// ~/.config/kempt/repo, with a leading ~ expanded via os.UserHomeDir.
func resolveRepoDir(flagVal string) (string, error) {
	if flagVal == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "kempt", "repo"), nil
	}
	if flagVal == "~" || strings.HasPrefix(flagVal, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(flagVal, "~"), "/")), nil
	}
	return flagVal, nil
}

// buildPickerInputs projects the manifest into deterministically-sorted picker
// inputs: profiles by name, then all packages as an unchecked checklist.
func buildPickerInputs(m *manifest.Manifest) ([]picker.Profile, []picker.Item) {
	pnames := make([]string, 0, len(m.Profiles))
	for name := range m.Profiles {
		pnames = append(pnames, name)
	}
	sort.Strings(pnames)
	profiles := make([]picker.Profile, 0, len(pnames))
	for _, n := range pnames {
		p := m.Profiles[n]
		profiles = append(profiles, picker.Profile{Name: p.Name, Description: p.Description, Packages: p.Packages})
	}

	inames := make([]string, 0, len(m.Packages))
	for name := range m.Packages {
		inames = append(inames, name)
	}
	sort.Strings(inames)
	items := make([]picker.Item, 0, len(inames))
	for _, n := range inames {
		pk := m.Packages[n]
		items = append(items, picker.Item{Name: pk.Name, Description: pk.Description})
	}
	return profiles, items
}
