package handlers

import (
	"net/url"
	"strings"
)

// SameRemote reports whether two git remotes name the same repository: the
// same host (any case) and path, whether written as https, http, ssh:// or
// scp-like git@host:path, with or without a trailing .git or /. A remote that
// is none of those forms, such as a local path, compares as written, less a
// trailing .git.
func SameRemote(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	ha, pa, oka := splitRemote(a)
	hb, pb, okb := splitRemote(b)
	if oka && okb {
		return strings.EqualFold(ha, hb) && pa == pb
	}
	return strings.TrimSuffix(a, ".git") == strings.TrimSuffix(b, ".git")
}

// splitRemote returns a remote's host and repository path, or false when it is
// not a URL or scp-like form.
func splitRemote(s string) (host, path string, ok bool) {
	switch {
	case strings.HasPrefix(s, "https://"), strings.HasPrefix(s, "http://"), strings.HasPrefix(s, "ssh://"):
		u, err := url.Parse(s)
		if err != nil || u.Hostname() == "" {
			return "", "", false
		}
		host, path = u.Hostname(), u.Path
	default:
		at := strings.Index(s, "@")
		colon := strings.Index(s, ":")
		if at <= 0 || colon < at+2 || strings.Contains(s[:colon], "/") {
			return "", "", false
		}
		host, path = s[at+1:colon], s[colon+1:]
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	path = strings.Trim(path, "/")
	if path == "" {
		return "", "", false
	}
	return host, path, true
}
