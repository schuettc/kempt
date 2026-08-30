package engine

import (
	"fmt"
	"io"

	"github.com/schuettc/kempt/internal/manifest"
)

var markers = map[Op]string{
	OpNoop:    "✓",
	OpChange:  "+",
	OpSkip:    "-",
	OpBlocked: "!",
}

// Render writes a human-readable plan to w. Each package gets a header line,
// followed by one indented line per step (marker + Detail). A footer summarises
// counts and, when there are changes, breaks them down by safety class.
func Render(p *Plan, w io.Writer) {
	var changes, ok, skipped, blocked int
	var software, files int

	for _, pp := range p.Packages {
		if pp.Skipped {
			fmt.Fprintf(w, "package %s (skipped: %s)\n", pp.Name, pp.Detail)
			continue
		}
		fmt.Fprintf(w, "package %s\n", pp.Name)
		for _, sr := range pp.Steps {
			fmt.Fprintf(w, "  %s %s\n", markers[sr.Delta.Op], sr.Delta.Detail)
			switch sr.Delta.Op {
			case OpNoop:
				ok++
			case OpChange:
				changes++
				switch sr.Step.Class() {
				case manifest.ClassSoftware:
					software++
				case manifest.ClassFiles:
					files++
				}
			case OpSkip:
				skipped++
			case OpBlocked:
				blocked++
			}
		}
	}

	fmt.Fprintf(w, "%d changes, %d ok, %d skipped, %d blocked\n", changes, ok, skipped, blocked)
	if changes > 0 {
		fmt.Fprintf(w, "software changes: %d, file changes: %d\n", software, files)
	}

	var allNotes []string
	for _, pp := range p.Packages {
		allNotes = append(allNotes, pp.Notes...)
	}
	if len(allNotes) > 0 {
		fmt.Fprintf(w, "manual follow-ups:\n")
		for _, note := range allNotes {
			fmt.Fprintf(w, "  - %s\n", note)
		}
	}
}
