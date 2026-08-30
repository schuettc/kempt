package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/release"
)

func makeTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "prefix/" + name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func checksums(asset string, body []byte) []byte {
	sum := sha256.Sum256(body)
	return []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), asset))
}

// exePath writes a placeholder exe into a tempdir and returns its path.
func exePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "kempt")
	if err := os.WriteFile(p, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUpdateReplacesBinary(t *testing.T) {
	exe := exePath(t)
	newBin := []byte("NEW-KEMPT-BINARY")
	asset := "kempt_darwin_arm64.tar.gz"
	targz := makeTarGz(t, "kempt", newBin)
	base := "https://github.com/schuettc/kempt/releases/download/v0.9.9/"
	rel := release.FakeReleases{
		Tags: map[string]string{"schuettc/kempt": "v0.9.9"},
		Files: map[string][]byte{
			base + asset:           targz,
			base + "checksums.txt": checksums(asset, targz),
		},
	}
	updated, ver, err := Update(Options{
		Repo:     "schuettc/kempt",
		Asset:    "kempt_{os}_{arch}.tar.gz",
		OS:       "darwin",
		Arch:     "arm64",
		Current:  "dev",
		ExePath:  exe,
		Releases: rel,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated {
		t.Fatalf("updated = false, want true")
	}
	if ver != "v0.9.9" {
		t.Fatalf("ver = %q, want v0.9.9", ver)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newBin) {
		t.Fatalf("exe content = %q, want %q", got, newBin)
	}
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0755", info.Mode().Perm())
	}
}

func TestUpdateNoopWhenCurrent(t *testing.T) {
	exe := exePath(t)
	rel := release.FakeReleases{
		Tags:  map[string]string{"schuettc/kempt": "vdev"},
		Files: map[string][]byte{},
	}
	updated, ver, err := Update(Options{
		Repo:     "schuettc/kempt",
		Asset:    "kempt_{os}_{arch}.tar.gz",
		OS:       "darwin",
		Arch:     "arm64",
		Current:  "dev",
		ExePath:  exe,
		Releases: rel,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated {
		t.Fatalf("updated = true, want false (already current)")
	}
	if ver != "" {
		t.Fatalf("ver = %q, want empty", ver)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != "OLD" {
		t.Fatalf("exe changed to %q, want untouched OLD", got)
	}
}

func TestUpdateChecksumMismatch(t *testing.T) {
	exe := exePath(t)
	asset := "kempt_darwin_arm64.tar.gz"
	targz := makeTarGz(t, "kempt", []byte("NEW"))
	base := "https://github.com/schuettc/kempt/releases/download/v0.9.9/"
	rel := release.FakeReleases{
		Tags: map[string]string{"schuettc/kempt": "v0.9.9"},
		Files: map[string][]byte{
			base + asset:           targz,
			base + "checksums.txt": []byte("deadbeef  " + asset + "\n"),
		},
	}
	_, _, err := Update(Options{
		Repo:     "schuettc/kempt",
		Asset:    "kempt_{os}_{arch}.tar.gz",
		OS:       "darwin",
		Arch:     "arm64",
		Current:  "dev",
		ExePath:  exe,
		Releases: rel,
	})
	if err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
	got, _ := os.ReadFile(exe)
	if string(got) != "OLD" {
		t.Fatalf("exe changed to %q, want untouched OLD", got)
	}
}
