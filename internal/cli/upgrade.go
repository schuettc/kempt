package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/schuettc/kempt/internal/engine"
	_ "github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/manifest"
)

func init() {
	Register(Command{Name: "upgrade", Summary: "upgrade installed tools to newer releases", Run: runUpgrade})
}

// runUpgrade upgrades installed download tools that are behind their pinned
// or latest-resolved version, reusing the download handler's verified Apply.
// It never edits the manifest.
func runUpgrade(args []string, out, errw io.Writer) error {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	manifestFlag := fs.String("manifest", "", "path to manifest")
	profileFlag := fs.String("profile", "", "profile to select")
	packagesFlag := fs.String("packages", "", "comma-separated package names")
	yes := YesFlag(fs, "apply without prompting")
	if err := ParseFlags(fs, args, out); err != nil {
		return err
	}
	only := map[string]bool{}
	for _, name := range fs.Args() {
		only[name] = true
	}

	_, selected, ctx, err := loadSelectedContext(*manifestFlag, *profileFlag, *packagesFlag, errw)
	if err != nil {
		return err
	}
	statuses, err := scanTools(ctx, selected)
	if err != nil {
		return err
	}

	var todo []toolStatus
	for _, s := range statuses {
		if len(only) != 0 && !only[s.Tool] {
			continue
		}
		if s.Err != nil {
			fmt.Fprintf(out, "skipping %s: could not resolve latest: %v\n", s.Tool, s.Err)
			continue
		}
		if s.Behind {
			todo = append(todo, s)
		}
	}
	if len(todo) == 0 {
		fmt.Fprintln(out, "everything up to date")
		return nil
	}
	for _, s := range todo {
		fmt.Fprintf(out, "%s  %s -> %s\n", s.Tool, s.Installed, s.Target)
	}
	if !*yes {
		fmt.Fprint(out, "upgrade these? [y/N] ")
		if !confirm() {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	h, _ := engine.HandlerFor("download")
	for _, s := range todo {
		// Version is the exact resolved target scanTools reported, so a
		// "latest" tool installs the version just resolved (no second
		// pointer read that could race a newer release).
		step := manifest.DownloadStep{Site: s.Site, Tool: s.Tool, Version: s.Target, Bin: s.Bin}
		if err := h.Apply(ctx, step); err != nil {
			return fmt.Errorf("upgrade %s: %w", s.Tool, err)
		}
		fmt.Fprintf(out, "upgraded %s to %s\n", s.Tool, s.Target)
	}
	return nil
}
