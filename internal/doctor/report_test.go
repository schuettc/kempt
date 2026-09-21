package doctor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReportCountsAndFailed(t *testing.T) {
	r := Report{Findings: []Finding{
		{Check: "d1", Severity: Warn, Detail: "x", Remediation: "y"},
		{Check: "d4", Severity: Error, Detail: "z", Remediation: "w"},
	}}
	e, wn, i := r.Counts()
	if e != 1 || wn != 1 || i != 0 {
		t.Fatalf("counts = %d,%d,%d", e, wn, i)
	}
	if !r.Failed(false) {
		t.Fatal("error should fail default threshold")
	}
	warnOnly := Report{Findings: []Finding{{Severity: Warn}}}
	if warnOnly.Failed(false) {
		t.Fatal("warn must not fail default threshold")
	}
	if !warnOnly.Failed(true) {
		t.Fatal("warn must fail under strict")
	}
}

func TestReportJSON(t *testing.T) {
	var b bytes.Buffer
	r := Report{Findings: []Finding{{Check: "d2", Severity: Warn, Package: "pi", Detail: "dup", Remediation: "apply"}}}
	if err := r.WriteJSON(&b); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Findings []map[string]any `json:"findings"`
		Summary  map[string]int   `json:"summary"`
	}
	if err := json.Unmarshal(b.Bytes(), &doc); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if doc.Findings[0]["severity"] != "warn" || doc.Summary["warn"] != 1 {
		t.Fatalf("bad json: %s", b.String())
	}
}

func TestReportHumanHealthy(t *testing.T) {
	var b bytes.Buffer
	Report{}.WriteHuman(&b)
	if !strings.Contains(b.String(), "healthy: no findings") {
		t.Fatalf("want healthy line, got %q", b.String())
	}
}
