package handlers

import (
	"net/url"
	"strings"
)

// SameRemote reports whether two git remotes name the same repository. It
// only looks past spelling where the evidence is unambiguous: https, http,
// ssh:// and scp-like user@host:path remotes match when host (any case),
// port (absent is the scheme's default), SSH user and path agree, less a
// trailing .git or /. An SSH remote whose user is git or absent is the
// hosting-service form of the https one; HTTPS credentials are ignored. A
// relative scp path is the URL path /path; an absolute one never equals it.
// Anything else, such as a local path or a URL with a query, compares exactly.
func SameRemote(a, b string) bool {
	ra, oka := parseRemote(a)
	rb, okb := parseRemote(b)
	if !oka || !okb {
		return a == b
	}
	if ra.port != rb.port || (ra.port != "" && ra.ssh != rb.ssh) {
		return false
	}
	return strings.EqualFold(ra.host, rb.host) && ra.user == rb.user && ra.path == rb.path
}

// remote is a remote's identity. port is "" when it is the scheme's default;
// user is "" for https and for an SSH user of git or none.
type remote struct {
	ssh                    bool
	user, host, port, path string
}

var defaultPorts = map[string]string{"ssh": "22", "https": "443", "http": "80"}

func parseRemote(s string) (remote, bool) {
	var r remote
	if i := strings.Index(s, "://"); i > 0 {
		scheme := s[:i]
		if _, known := defaultPorts[scheme]; !known {
			return r, false
		}
		u, err := url.Parse(s)
		if err != nil || u.Hostname() == "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(s, "#") {
			return r, false
		}
		r.ssh, r.host, r.port, r.path = scheme == "ssh", u.Hostname(), u.Port(), u.Path
		if r.port == defaultPorts[scheme] {
			r.port = ""
		}
		if r.ssh && u.User != nil {
			r.user = u.User.Username()
		}
	} else {
		at := strings.Index(s, "@")
		colon := strings.Index(s, ":")
		if at <= 0 || colon < at+2 || strings.ContainsAny(s[:colon], "/ ") {
			return r, false
		}
		r.ssh, r.user, r.host, r.path = true, s[:at], s[at+1:colon], s[colon+1:]
		if strings.HasPrefix(r.path, "/") {
			r.path = "abs:" + r.path // home-relative and absolute paths never meet
		} else {
			r.path = "/" + r.path
		}
	}
	if r.user == "git" {
		r.user = ""
	}
	r.path = strings.TrimRight(strings.TrimSuffix(strings.TrimRight(r.path, "/"), ".git"), "/")
	if r.path == "" || r.path == "abs:" || !strings.HasPrefix(r.path, "/") && !strings.HasPrefix(r.path, "abs:/") {
		return r, false
	}
	return r, true
}
