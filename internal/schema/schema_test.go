package schema

import (
	"encoding/json"
	"testing"
)

func TestJSONIsValid(t *testing.T) {
	if !json.Valid(JSON()) {
		t.Fatal("schema.JSON() is not valid JSON")
	}
}

func TestTopLevelShape(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal(JSON(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["$schema"]; !ok {
		t.Error("missing $schema")
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing properties object")
	}
	if _, ok := props["packages"]; !ok {
		t.Error("missing properties.packages")
	}
}

// packageProperties navigates the schema to the properties object of a package.
func packageProperties(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(JSON(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// $defs.package.properties
	defs, ok := m["$defs"].(map[string]any)
	if !ok {
		t.Fatal("missing $defs")
	}
	pkg, ok := defs["package"].(map[string]any)
	if !ok {
		t.Fatal("missing $defs.package")
	}
	props, ok := pkg["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing $defs.package.properties")
	}
	return props
}

func TestPackageStepKindsPresent(t *testing.T) {
	props := packageProperties(t)
	kinds := []string{"install", "github-release", "download", "git-clone", "service", "symlink", "json-merge", "toml-merge", "line-in-file", "verify"}
	for _, k := range kinds {
		if _, ok := props[k]; !ok {
			t.Errorf("package schema missing step kind property %q", k)
		}
	}
}
