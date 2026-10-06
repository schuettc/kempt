package handlers

import "testing"

func TestSameRemote(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"git@github.com:owner/repo.git", "https://github.com/owner/repo.git", true},
		{"https://github.com/owner/repo.git", "git@github.com:owner/repo.git", true},
		{"git@github.com:owner/repo", "https://github.com/owner/repo.git", true},
		{"git@github.com:owner/repo.git", "https://github.com/owner/repo", true},
		{"https://github.com/owner/repo/", "git@github.com:owner/repo.git", true},
		{"ssh://git@github.com/owner/repo.git", "https://github.com/owner/repo", true},
		{"ssh://git@github.com:22/owner/repo.git", "git@github.com:owner/repo", true},
		{"http://github.com/owner/repo", "https://github.com/owner/repo", true},
		{"https://GitHub.com/owner/repo", "git@github.com:owner/repo.git", true},
		{"git@github.com:other/repo.git", "https://github.com/owner/repo.git", false},
		{"git@github.com:owner/other.git", "https://github.com/owner/repo.git", false},
		{"git@gitlab.com:owner/repo.git", "https://github.com/owner/repo.git", false},
		{"https://github.com/Owner/repo", "https://github.com/owner/repo", false},
		{"/srv/git/repo", "/srv/git/repo", true},
		{"/srv/git/repo", "/srv/git/other", false},
	}
	for _, c := range cases {
		if got := SameRemote(c.a, c.b); got != c.want {
			t.Errorf("SameRemote(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
