package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDoctorRegistered(t *testing.T) {
	var out, errw bytes.Buffer
	if code := Dispatch([]string{"commands", "--json"}, &out, &errw); code != 0 {
		t.Fatalf("commands exit=%d", code)
	}
	if !strings.Contains(out.String(), "\"doctor\"") {
		t.Fatalf("doctor not in commands index: %s", out.String())
	}
}
