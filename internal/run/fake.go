package run

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrForTest is a sentinel error for scripting failures in FakeRunner
// Responses.
var ErrForTest = errors.New("forced error for tests")

// FakeRunner is a scripted Runner for use in tests.
//
// Key contract for Responses:
//   - Run calls:      strings.TrimSpace(name + " " + strings.Join(args, " "))
//     Zero-arg calls use the bare name (e.g. "git"); multi-arg calls use name
//     and args joined by single spaces (e.g. "git status").
//   - LookPath calls: "lookpath " + name
//
// A missing key in Responses causes Run to return an "unscripted command" error
// and LookPath to return ("", exec.ErrNotFound). All invoked keys are appended
// to Calls in order.
//
// Sequences (Run only) scripts a key whose answer changes between calls, such
// as an inventory read before and after an install: successive calls take the
// responses in order and the last one repeats. A key in Sequences wins over
// the same key in Responses.
type FakeRunner struct {
	Responses map[string]Response   // keyed by the formats above
	Sequences map[string][]Response // Run keys answered in call order
	Calls     []string              // recorded keys in invocation order
}

// Response holds the scripted result for a FakeRunner entry.
type Response struct {
	Stdout string
	Err    error
}

// Run records the call and returns the scripted response, or an error if the
// key is not found in Responses.
func (f *FakeRunner) Run(name string, args ...string) (string, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.Calls = append(f.Calls, key)
	if seq := f.Sequences[key]; len(seq) > 0 {
		resp := seq[0]
		if len(seq) > 1 {
			f.Sequences[key] = seq[1:]
		}
		return resp.Stdout, resp.Err
	}
	resp, ok := f.Responses[key]
	if !ok {
		return "", fmt.Errorf("unscripted command: %s", key)
	}
	return resp.Stdout, resp.Err
}

// LookPath records the call and returns the scripted response, or
// ("", exec.ErrNotFound) if the key is not found in Responses.
func (f *FakeRunner) LookPath(name string) (string, error) {
	key := "lookpath " + name
	f.Calls = append(f.Calls, key)
	resp, ok := f.Responses[key]
	if !ok {
		return "", exec.ErrNotFound
	}
	return resp.Stdout, resp.Err
}
