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
		Name:    "status",
		Summary: "show cached refresh status",
		Run:     runStatus,
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

	// Build the body from segments.
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
		fmt.Fprintln(out, "kempt: up to date")
	} else {
		fmt.Fprintf(out, "kempt: %s · kempt update\n", body)
	}
	return nil
}
