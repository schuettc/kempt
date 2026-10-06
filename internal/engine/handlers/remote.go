package handlers

import (
	"net/url"
	"strings"
)

// SameRemote reports whether two git remotes name the same repository. Only
// the plain hosting-service forms are normalised: https:// or http://, and SSH
// as user git or none (ssh://git@host/owner/repo or git@host:owner/repo), at
// the scheme's default port, with no percent-escape, query or fragment. Two of
// those match when host (any case) and path agree, less a trailing .git or /.
// Anything else, such as a local path, another SSH user, a non-default port or
// an absolute scp path, compares exactly.
func SameRemote(a, b string) bool {
	ha, pa, oka := hostingRemote(a)
	hb, pb, okb := hostingRemote(b)
	if !oka || !okb {
		return a == b
	}
	return strings.EqualFold(ha, hb) && pa == pb
}

var defaultPorts = map[string]string{"ssh": "22", "https": "443", "http": "80"}

// hostingRemote returns a plain hosting-form remote's host and /path, or false
// for any other remote.
func hostingRemote(s string) (host, path string, ok bool) {
	if strings.ContainsAny(s, "%?#") {
		return "", "", false
	}
	if i := strings.Index(s, "://"); i > 0 {
		scheme := s[:i]
		def, known := defaultPorts[scheme]
		u, err := url.Parse(s)
		if !known || err != nil || u.Hostname() == "" {
			return "", "", false
		}
		if p := u.Port(); p != "" && p != def {
			return "", "", false
		}
		if scheme == "ssh" && u.User != nil && u.User.Username() != "git" {
			return "", "", false
		}
		host, path = u.Hostname(), u.Path
	} else {
		colon := strings.Index(s, ":")
		if colon <= 0 || strings.ContainsAny(s[:colon], "/ ") {
			return "", "", false
		}
		host, path = s[:colon], "/"+s[colon+1:]
		if user, h, found := strings.Cut(host, "@"); found {
			if user != "git" {
				return "", "", false
			}
			host = h
		}
		if host == "" || strings.HasPrefix(path, "//") {
			return "", "", false // absolute scp paths are not the hosting form
		}
	}
	path = strings.TrimRight(strings.TrimSuffix(strings.TrimRight(path, "/"), ".git"), "/")
	if path == "" {
		return "", "", false
	}
	return host, path, true
}
