package run

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Runner abstracts command execution and binary lookup.
type Runner interface {
	Run(name string, args ...string) (stdout string, err error) // err wraps stderr text on failure
	LookPath(name string) (string, error)
}

// RealRunner is an os/exec-backed Runner.
type RealRunner struct{}

// Run executes the named command with the given args. It captures stdout and
// returns it on success. On non-zero exit it returns an error of the form
// "<name>: <exitError>: <stderr>".
func (r RealRunner) Run(name string, args ...string) (string, error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, stderrBuf.String())
	}
	return stdoutBuf.String(), nil
}

// LookPath reports the path of the named binary using exec.LookPath.
func (r RealRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}
