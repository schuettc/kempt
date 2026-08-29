package release

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Releases is the interface for querying and downloading releases.
// Wired in Task 7; declared here now to avoid an import cycle since
// machine.Context declares a Releases field.
type Releases interface {
	LatestTag(repo string) (string, error)
	Download(url string) ([]byte, error)
}

const defaultBase = "https://github.com"

// RealReleases resolves and downloads GitHub release assets over HTTP.
// The zero value is usable: Client defaults to http.DefaultClient and Base
// defaults to https://github.com. Tests point Base at an httptest server.
type RealReleases struct {
	Client *http.Client
	Base   string // defaults to https://github.com
}

func (r RealReleases) base() string {
	if r.Base != "" {
		return strings.TrimRight(r.Base, "/")
	}
	return defaultBase
}

// LatestTag issues a GET to <Base>/<repo>/releases/latest and captures the
// redirect Location without following it. The tag is the path segment after
// "/tag/". A non-redirect response or a Location lacking that segment is an
// error.
func (r RealReleases) LatestTag(repo string) (string, error) {
	// Clone the client so we can install a capturing CheckRedirect without
	// mutating the caller's client.
	base := http.DefaultClient
	if r.Client != nil {
		base = r.Client
	}
	c := &http.Client{
		Transport: base.Transport,
		Jar:       base.Jar,
		Timeout:   base.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	url := fmt.Sprintf("%s/%s/releases/latest", r.base(), repo)
	resp, err := c.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("latest tag for %s: no redirect from %s (status %d)", repo, url, resp.StatusCode)
	}
	idx := strings.Index(loc, "/tag/")
	if idx < 0 {
		return "", fmt.Errorf("latest tag for %s: redirect %q has no /tag/ segment", repo, loc)
	}
	tag := loc[idx+len("/tag/"):]
	if i := strings.IndexByte(tag, '/'); i >= 0 {
		tag = tag[:i]
	}
	if tag == "" {
		return "", fmt.Errorf("latest tag for %s: empty tag in redirect %q", repo, loc)
	}
	return tag, nil
}

// Download issues a GET and returns the body. A non-200 response is an error.
func (r RealReleases) Download(url string) ([]byte, error) {
	c := http.DefaultClient
	if r.Client != nil {
		c = r.Client
	}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// FakeReleases is an in-memory Releases for tests. Tags maps repo → tag and
// Files maps url → body.
type FakeReleases struct {
	Tags  map[string]string
	Files map[string][]byte
}

func (f FakeReleases) LatestTag(repo string) (string, error) {
	tag, ok := f.Tags[repo]
	if !ok {
		return "", fmt.Errorf("no tag for repo %s", repo)
	}
	return tag, nil
}

func (f FakeReleases) Download(url string) ([]byte, error) {
	body, ok := f.Files[url]
	if !ok {
		return nil, fmt.Errorf("no file for url %s", url)
	}
	return body, nil
}
