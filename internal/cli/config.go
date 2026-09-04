package cli

import (
	"fmt"
	"io"
	"strconv"
)

func init() {
	Register(Command{
		Name:     "config",
		Summary:  "get or set kempt configuration",
		Synopsis: "config <get|set> [key] [value]",
		Help:     "Gets or sets kempt configuration (the saved manifest path, profile, and package selection).",
		Run:      runConfig,
	})
}

// autoApplyFilesKey is the only configuration key in this phase.
const autoApplyFilesKey = "auto-apply-files"

func runConfig(args []string, out, errw io.Writer) error {
	if len(args) < 1 {
		return UsageError{Msg: "usage: kempt config get|set <key> [value]"}
	}
	switch args[0] {
	case "get":
		return configGet(args[1:], out)
	case "set":
		return configSet(args[1:], out)
	default:
		return UsageError{Msg: fmt.Sprintf("unknown subcommand %q; want get or set", args[0])}
	}
}

func configGet(args []string, out io.Writer) error {
	if len(args) != 1 {
		return UsageError{Msg: "usage: kempt config get <key>"}
	}
	if args[0] != autoApplyFilesKey {
		return UsageError{Msg: fmt.Sprintf("unknown key %q", args[0])}
	}
	st, _, err := loadState()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%t\n", st.AutoApplyFiles)
	return nil
}

func configSet(args []string, out io.Writer) error {
	if len(args) != 2 {
		return UsageError{Msg: "usage: kempt config set <key> <value>"}
	}
	if args[0] != autoApplyFilesKey {
		return UsageError{Msg: fmt.Sprintf("unknown key %q", args[0])}
	}
	v, err := strconv.ParseBool(args[1])
	if err != nil {
		return UsageError{Msg: fmt.Sprintf("invalid bool %q for %s", args[1], autoApplyFilesKey)}
	}
	st, _, err := loadState()
	if err != nil {
		return err
	}
	st.AutoApplyFiles = v
	if err := saveState(st); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s = %t\n", autoApplyFilesKey, v)
	return nil
}
