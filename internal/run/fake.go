package run

import (
	"fmt"
	"os/exec"
	"strings"
)

// FakeRunner is a scripted Runner for use in tests.
//
// Key format for Responses:
//   - For Run calls:      name + " " + strings.Join(args, " ")
//   - For LookPath calls: "lookpath " + name
//
// A missing key in Responses causes Run to return an "unscripted command" error
// and LookPath to return ("", exec.ErrNotFound). All invoked keys are appended
// to Calls in order.
type FakeRunner struct {
	Responses map[string]Response // keyed by the formats above
	Calls     []string            // recorded keys in invocation order
}

// Response holds the scripted result for a FakeRunner entry.
type Response struct {
	Stdout string
	Err    error
}

// Run records the call and returns the scripted response, or an error if the
// key is not found in Responses.
func (f *FakeRunner) Run(name string, args ...string) (string, error) {
	key := name
	if len(args) > 0 {
		key = name + " " + strings.Join(args, " ")
	}
	f.Calls = append(f.Calls, key)
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
