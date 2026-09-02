package cli

import (
	"encoding/json"
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
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
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

	if *jsonFlag {
		return printOutdatedJSON(out, statuses)
	}

	behind := 0
	errored := 0
	for _, s := range statuses {
		switch {
		case s.Err != nil:
			fmt.Fprintf(out, "%s  ? (could not resolve latest: %v)\n", s.Tool, s.Err)
			errored++
		case s.Behind:
			fmt.Fprintf(out, "%s  %s -> %s  (%s)\n", s.Tool, s.Installed, s.Target, s.Mode)
			behind++
		}
	}
	if behind == 0 && errored == 0 {
		fmt.Fprintln(out, "everything up to date")
	}
	return nil
}

// outdatedJSON is the --json shape for one tool's status.
type outdatedJSON struct {
	Tool      string `json:"tool"`
	Installed string `json:"installed"`
	Target    string `json:"target"`
	Mode      string `json:"mode"`
	Behind    bool   `json:"behind"`
	Error     string `json:"error,omitempty"`
}

func printOutdatedJSON(out io.Writer, statuses []toolStatus) error {
	rows := make([]outdatedJSON, 0, len(statuses))
	for _, s := range statuses {
		row := outdatedJSON{Tool: s.Tool, Installed: s.Installed, Target: s.Target, Mode: s.Mode, Behind: s.Behind}
		if s.Err != nil {
			row.Error = s.Err.Error()
		}
		rows = append(rows, row)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}
