// Package doctor holds read-only machine-vs-manifest health checks composed by
// Run and surfaced by the `kempt doctor` command.
package doctor

type Severity int

const (
	Info Severity = iota
	Warn
	Error
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warn:
		return "warn"
	default:
		return "info"
	}
}

// Finding is one read-only diagnosis. Remediation is a concrete next step.
type Finding struct {
	Check       string
	Package     string
	Detail      string
	Remediation string
	Severity    Severity
}
