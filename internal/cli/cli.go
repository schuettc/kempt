package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/schuettc/kempt/internal/version"
)

type UsageError struct{ Msg string }

func (e UsageError) Error() string { return e.Msg }

type Command struct {
	Name    string
	Summary string
	Run     func(args []string, out, errw io.Writer) error
}

var registry = map[string]Command{}

func Register(cmd Command) { registry[cmd.Name] = cmd }

func init() {
	Register(Command{Name: "version", Summary: "print kempt version",
		Run: func(args []string, out, errw io.Writer) error {
			fmt.Fprintf(out, "kempt %s\n", version.String())
			return nil
		}})
	Register(Command{Name: "help", Summary: "show usage",
		Run: func(args []string, out, errw io.Writer) error {
			usage(out)
			return nil
		}})
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: kempt <command> [args]")
	fmt.Fprintln(w, "\ncommands:")
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "  %-10s %s\n", n, registry[n].Summary)
	}
}

func Dispatch(args []string, out, errw io.Writer) int {
	if len(args) == 0 {
		usage(errw)
		return 2
	}
	name := args[0]
	switch name {
	case "--version", "-v":
		name = "version"
	case "--help", "-h":
		name = "help"
	}
	cmd, ok := registry[name]
	if !ok {
		fmt.Fprintf(errw, "kempt: unknown command %q\n\n", name)
		usage(errw)
		return 2
	}
	if err := cmd.Run(args[1:], out, errw); err != nil {
		var ue UsageError
		if errors.As(err, &ue) {
			fmt.Fprintf(errw, "kempt %s: %s\n", name, ue.Msg)
			return 2
		}
		fmt.Fprintf(errw, "kempt %s: %v\n", name, err)
		return 1
	}
	return 0
}
