package jsonutil

import "testing"

func TestSpecIdentity(t *testing.T) {
	cases := map[string]string{
		"npm:pi-quiet":                      "pi-quiet",
		"npm:pi-quiet@0.2.0":                "pi-quiet",
		"npm:@scope/name":                   "@scope/name",
		"npm:@scope/name@1.2.3":             "@scope/name",
		"git:github.com/obra/superpowers":   "github.com/obra/superpowers",
		"git:github.com/obra/superpowers@v1": "github.com/obra/superpowers",
		"/abs/local/path":                   "/abs/local/path",
	}
	for in, want := range cases {
		if got := SpecIdentity(in); got != want {
			t.Errorf("SpecIdentity(%q) = %q; want %q", in, got, want)
		}
	}
}
