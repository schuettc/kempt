package doctor

import (
	"encoding/json"
	"fmt"
	"io"
)

type Report struct{ Findings []Finding }

func (r Report) Counts() (errs, warns, infos int) {
	for _, f := range r.Findings {
		switch f.Severity {
		case Error:
			errs++
		case Warn:
			warns++
		default:
			infos++
		}
	}
	return
}

// Failed reports whether the run should exit non-zero: any error, or any warn
// when strict.
func (r Report) Failed(strict bool) bool {
	e, w, _ := r.Counts()
	if e > 0 {
		return true
	}
	return strict && w > 0
}

func (r Report) WriteHuman(w io.Writer) {
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "healthy: no findings")
		return
	}
	for _, sev := range []Severity{Error, Warn, Info} {
		for _, f := range r.Findings {
			if f.Severity != sev {
				continue
			}
			pkg := ""
			if f.Package != "" {
				pkg = " [" + f.Package + "]"
			}
			fmt.Fprintf(w, "  %-5s %s%s %s\n", f.Severity, f.Check, pkg, f.Detail)
			fmt.Fprintf(w, "        → %s\n", f.Remediation)
		}
	}
	e, wn, i := r.Counts()
	fmt.Fprintf(w, "%d errors, %d warnings, %d info\n", e, wn, i)
}

func (r Report) WriteJSON(w io.Writer) error {
	e, wn, i := r.Counts()
	type jf struct {
		Check       string `json:"check"`
		Severity    string `json:"severity"`
		Package     string `json:"package,omitempty"`
		Detail      string `json:"detail"`
		Remediation string `json:"remediation"`
	}
	out := struct {
		Findings []jf           `json:"findings"`
		Summary  map[string]int `json:"summary"`
	}{Summary: map[string]int{"error": e, "warn": wn, "info": i}}
	for _, f := range r.Findings {
		out.Findings = append(out.Findings, jf{f.Check, f.Severity.String(), f.Package, f.Detail, f.Remediation})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
