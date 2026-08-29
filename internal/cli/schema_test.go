package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaCommand(t *testing.T) {
	var out, errw bytes.Buffer
	code := Dispatch([]string{"schema"}, &out, &errw)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errw.String())
	}
	if !json.Valid(out.Bytes()) {
		t.Fatal("output is not valid JSON")
	}
	if !strings.Contains(out.String(), "packages") {
		t.Error("output does not contain \"packages\"")
	}
}

func TestSchemaExtraArgs(t *testing.T) {
	var out, errw bytes.Buffer
	code := Dispatch([]string{"schema", "extra"}, &out, &errw)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}
