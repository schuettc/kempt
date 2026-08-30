package machine

import (
	"testing"
)

func TestExpand(t *testing.T) {
	c := &Context{Home: "/h", RepoDir: "/r"}

	cases := []struct {
		input string
		want  string
	}{
		{"~", "/h"},
		{"~/a/b", "/h/a/b"},
		{"/abs", "/abs"},
		{"/abs/path", "/abs/path"},
		{"rel/x", "/r/rel/x"},
		{"foo", "/r/foo"},
		{"~x", "/r/~x"}, // tilde not followed by slash is repo-relative, not home
	}

	for _, tc := range cases {
		got := c.Expand(tc.input)
		if got != tc.want {
			t.Errorf("Expand(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}
