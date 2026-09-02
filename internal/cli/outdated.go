package cli

import (
	"flag"
	"fmt"
	"io"

	_ "github.com/schuettc/kempt/internal/engine/handlers"
)

func init() {
	Register(Command{Name: "outdated", Summary: "list installed tools with newer releases", Run: runOutdated})
}

func runOutdated(args []string, out, errw io.Writer) error {
	fs := flag.NewFlagSet("outdated", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	manifestFlag := fs.String("manifest", "", "path to manifest")
	profileFlag := fs.String("profile", "", "profile to select")
	packagesFlag := fs.String("packages", "", "comma-separated package names")
	if err := fs.Parse(args); err != nil {
		return UsageError{Msg: err.Error()}
	}

	_, selected, ctx, err := loadSelectedContext(*manifestFlag, *profileFlag, *packagesFlag, errw)
	if err != nil {
		return err
	}

	statuses, err := scanTools(ctx, selected)
	if err != nil {
		return err
	}
	behind := 0
	for _, s := range statuses {
		if s.Behind {
			fmt.Fprintf(out, "%s  %s -> %s  (%s)\n", s.Tool, s.Installed, s.Target, s.Mode)
			behind++
		}
	}
	if behind == 0 {
		fmt.Fprintln(out, "everything up to date")
	}
	return nil
}
