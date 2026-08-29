package release

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRealReleasesLatestTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/schuettc/x/releases/latest" {
			http.Redirect(w, r, "/schuettc/x/releases/tag/v1.2.3", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	rr := RealReleases{Client: srv.Client(), Base: srv.URL}
	tag, err := rr.LatestTag("schuettc/x")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.2.3" {
		t.Fatalf("tag = %q, want v1.2.3", tag)
	}
}

func TestRealReleasesLatestTagMalformedRedirect(t *testing.T) {
	// Location header present but has no /tag/ segment.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/schuettc/x/releases/latest" {
			http.Redirect(w, r, "/schuettc/x/releases/expanded_assets/v1.2.3", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	rr := RealReleases{Client: srv.Client(), Base: srv.URL}
	_, err := rr.LatestTag("schuettc/x")
	if err == nil {
		t.Fatal("expected error for redirect without /tag/ segment")
	}
	if !strings.Contains(err.Error(), "/tag/") {
		t.Fatalf("error %q should mention missing /tag/ segment", err.Error())
	}
}

func TestRealReleasesLatestTagNoRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rr := RealReleases{Client: srv.Client(), Base: srv.URL}
	if _, err := rr.LatestTag("schuettc/x"); err == nil {
		t.Fatal("expected error on non-redirect response")
	}
}

func TestRealReleasesDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.Write([]byte("hello"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	rr := RealReleases{Client: srv.Client(), Base: srv.URL}

	body, err := rr.Download(srv.URL + "/ok")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" {
		t.Fatalf("body = %q, want hello", body)
	}

	if _, err := rr.Download(srv.URL + "/missing"); err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestFakeReleasesLatestTagMissingRepo(t *testing.T) {
	f := FakeReleases{Tags: map[string]string{}}
	_, err := f.LatestTag("example/missing")
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
	if !strings.Contains(err.Error(), "example/missing") {
		t.Fatalf("error %q should name the repo", err.Error())
	}
}

func TestFakeReleasesDownloadMissing(t *testing.T) {
	f := FakeReleases{Files: map[string][]byte{}}
	_, err := f.Download("https://example.com/x")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "https://example.com/x") {
		t.Fatalf("error %q should name the url", got)
	}
}
