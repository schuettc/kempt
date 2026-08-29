package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func init() { engine.RegisterHandler(serviceHandlerImpl{}) }

// plistTemplate is the launchd agent template. %s = Label, %s = the
// ProgramArguments block (one "\t\t<string>ARG</string>\n" line per element).
const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>KeepAlive</key>
	<true/>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`

// xmlEscaper escapes the XML metacharacters that can appear in program args.
var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// renderPlist produces the exact launchd plist for a service step.
func renderPlist(st manifest.ServiceStep) string {
	var args strings.Builder
	for _, a := range st.Program {
		args.WriteString("\t\t<string>")
		args.WriteString(xmlEscaper.Replace(a))
		args.WriteString("</string>\n")
	}
	return fmt.Sprintf(plistTemplate, st.Label, args.String())
}

// serviceHandlerImpl manages a launchd user agent (darwin-only backend).
type serviceHandlerImpl struct{}

func (serviceHandlerImpl) Kind() string { return "service" }

func plistPathFor(ctx *machine.Context, label string) string {
	return filepath.Join(ctx.Home, "Library", "LaunchAgents", label+".plist")
}

func (serviceHandlerImpl) Inspect(ctx *machine.Context, s manifest.Step) (engine.Delta, error) {
	st := s.(manifest.ServiceStep)
	base := fmt.Sprintf("service %s", st.Label)

	if ctx.OS != "darwin" {
		return engine.Delta{Op: engine.OpBlocked, Detail: base + " (launchd backend requires darwin)"}, nil
	}

	want := renderPlist(st)
	path := plistPathFor(ctx, st.Label)
	cur, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return engine.Delta{Op: engine.OpChange, Detail: base + " (install agent)"}, nil
		}
		return engine.Delta{}, err
	}
	if string(cur) != want {
		return engine.Delta{Op: engine.OpChange, Detail: base + " (update agent)"}, nil
	}
	return engine.Delta{Op: engine.OpNoop, Detail: base}, nil
}

func (serviceHandlerImpl) Apply(ctx *machine.Context, s manifest.Step) error {
	st := s.(manifest.ServiceStep)
	want := renderPlist(st)
	path := plistPathFor(ctx, st.Label)

	// Determine whether the on-disk content already matches. When it does,
	// applying is a no-op: no write, no reload.
	cur, err := os.ReadFile(path)
	switch {
	case err == nil:
		if string(cur) == want {
			return nil
		}
	case !os.IsNotExist(err):
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		return err
	}

	// Reload the agent: bootout (ignore error — it may not be loaded) then
	// bootstrap (error propagates).
	domain := fmt.Sprintf("gui/%d", ctx.UID)
	_, _ = ctx.Runner.Run("launchctl", "bootout", domain, path)
	if _, err := ctx.Runner.Run("launchctl", "bootstrap", domain, path); err != nil {
		return err
	}
	return nil
}
