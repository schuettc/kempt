package cli

import (
	"io"

	"github.com/schuettc/kempt/internal/version"
	tools "github.com/schuettc/tools-common"
)

// Command and UsageError are aliases onto the shared tools package so the
// per-command files' Command{...} literals and UsageError{...} returns compile
// unchanged.
type Command = tools.Command
type UsageError = tools.UsageError

// ParseFlags and YesFlag are the shared family CLI helpers, aliased so each
// command file calls them unqualified (as it does Register/Command/UsageError).
// ParseFlags gives every subcommand a real -h/--help flag listing and maps
// flag.ErrHelp to a clean exit through Dispatch; YesFlag registers -yes and its
// -y shorthand once.
var (
	ParseFlags = tools.ParseFlags
	YesFlag    = tools.YesFlag
	SplitArgs  = tools.SplitArgs
)

// commands holds the per-command registrations from the init() files. tools
// auto-registers version/help/update; those live on the App, not here.
var commands []tools.Command

// Register appends a command. Each command file's init() calls this unchanged.
func Register(cmd Command) { commands = append(commands, cmd) }

// Dispatch builds a tools.App, registers kempt's commands plus its richer
// update override, and delegates routing.
func Dispatch(args []string, out, errw io.Writer) int {
	app := tools.New(tools.Config{
		Name:   "kempt",
		Domain: "kempt.tools",
		Version: tools.Version{
			Number: version.Number(),
			Commit: version.Commit(),
			Date:   version.Date(),
		},
	})
	for _, c := range commands {
		app.Register(c)
	}
	// Register update AFTER the slice and as a closure capturing app: this
	// overrides tools' built-in self-update-only update with kempt's
	// pull+self-update+converge flow.
	app.Register(tools.Command{
		Name:     "update",
		Summary:  "pull the repo, self-update the binary, and converge",
		Synopsis: "update",
		Help:     "Pulls the config repo, self-updates the kempt binary from kempt.tools/dl, and applies.",
		Run: func(args []string, out, errw io.Writer) error {
			return runUpdate(app, args, out, errw)
		},
	})
	return app.Dispatch(args, out, errw)
}
