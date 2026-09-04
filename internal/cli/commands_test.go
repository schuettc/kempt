package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCommandsJSONCoversEveryCommand(t *testing.T) {
	var out, errw bytes.Buffer
	if code := Dispatch([]string{"commands", "--json"}, &out, &errw); code != 0 {
		t.Fatalf("exit = %d; err=%s", code, errw.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	names := map[string]bool{}
	for _, c := range got {
		names[c["name"].(string)] = true
	}
	for _, want := range []string{"apply", "plan", "upgrade", "init", "refresh", "adopt", "drop", "dump", "outdated", "verify", "update", "config", "lint", "new", "status", "schema", "help", "man", "commands", "version"} {
		if !names[want] {
			t.Fatalf("commands --json missing %q; got %v", want, names)
		}
	}
}

func TestManRenders(t *testing.T) {
	var out, errw bytes.Buffer
	if code := Dispatch([]string{"man"}, &out, &errw); code != 0 {
		t.Fatalf("exit = %d; err=%s", code, errw.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(".TH KEMPT 1")) {
		t.Fatalf("man output wrong: %s", out.String())
	}
}
