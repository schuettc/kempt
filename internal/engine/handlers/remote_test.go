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
		{"/srv/repo.git", "/srv/repo", false},
		{" /srv/repo", "/srv/repo", false},
		{"ssh://git@host:2222/owner/repo", "ssh://git@host:3333/owner/repo", false},
		{"ssh://git@host:2222/owner/repo", "https://host/owner/repo", false},
		{"ssh://git@host:22/owner/repo", "ssh://git@host/owner/repo", true},
		{"https://host:443/owner/repo", "https://host/owner/repo", true},
		{"https://host:8443/owner/repo", "https://host/owner/repo", false},
		{"git@host:repo", "git@host:/repo", false},
		{"git@host:/repo", "git@host:/repo", true},
		{"alice@host:repo", "bob@host:repo", false},
		{"alice@host:repo", "alice@host:repo.git", true},
		{"ssh://alice@host/owner/repo", "ssh://bob@host/owner/repo", false},
		{"ssh://host/owner/repo", "https://host/owner/repo", true},
		{"ssh://host/owner/repo", "git@host:owner/repo", true},
		{"deploy@host:owner/repo", "https://host/owner/repo", false},
		{"https://user:token@host/owner/repo", "git@host:owner/repo.git", true},
		{"https://host/owner/repo?ref=x", "https://host/owner/repo?ref=x", true},
		{"https://host/owner/repo?ref=x", "https://host/owner/repo", false},
		{"https://host/owner/repo#x", "https://host/owner/repo", false},
		{"/srv/git/repo", "/srv/git/repo", true},
		{"/srv/git/repo", "/srv/git/other", false},
	}
	for _, c := range cases {
		if got := SameRemote(c.a, c.b); got != c.want {
			t.Errorf("SameRemote(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
