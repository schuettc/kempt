package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/schuettc/kempt/internal/state"
)

// statusStore is a seam for tests to inject a custom Store without touching
// the real XDG data directory.
var statusStore = func() (*state.Store, error) {
	return state.DefaultStore()
}

func init() {
	Register(Command{
		Name:     "status",
		Summary:  "show cached refresh status",
		Synopsis: "status",
		Help:     "Shows cached refresh status for the current selection.",
		Run:      runStatus,
	})
}

func runStatus(args []string, out, errw io.Writer) error {
	store, err := statusStore()
	if err != nil {
		fmt.Fprintln(out, "kempt: status unavailable")
		return nil
	}
	st, existed, err := store.LoadStatus()
	if err != nil {
		fmt.Fprintln(out, "kempt: no status yet — run kempt refresh")
		return nil
	}
	if !existed {
		fmt.Fprintln(out, "kempt: no status yet — run kempt refresh")
		return nil
	}

	fmt.Fprintln(out, formatStatus(st))
	return nil
}

// formatStatus renders a cached Status as the single human summary line (no
// trailing newline) shared by `kempt status` and `kempt refresh`.
func formatStatus(st *state.Status) string {
	var parts []string
	if st.Behind > 0 {
		parts = append(parts, fmt.Sprintf("%d behind", st.Behind))
	}
	if st.FileChanges > 0 || st.SoftwareChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d file, %d software changes pending",
			st.FileChanges, st.SoftwareChanges))
	}

	body := strings.Join(parts, " · ")

	if st.Blocked > 0 {
		if body == "" {
			body = fmt.Sprintf("%d blocked", st.Blocked)
		} else {
			body += fmt.Sprintf("; %d blocked", st.Blocked)
		}
	}

	if body == "" {
		return "kempt: up to date"
	}
	return fmt.Sprintf("kempt: %s · kempt update", body)
}
