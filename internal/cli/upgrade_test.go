package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// fakeTarball builds a gzip'd tar containing a single file named name with
// the given content, and returns (tarball, sha256 sidecar) in the
// "shasum -a 256" sidecar format the download handler verifies. Mirrors
// makeTarGz/checksums in internal/engine/handlers/githubrelease_test.go.
func fakeTarball(t *testing.T, name, content string) (tarball, sidecar string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte(content)
	hdr := &tar.Header{Name: "prefix/" + name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	tarball = buf.String()

	assetName := name + "_darwin_arm64.tar.gz"
	sum := sha256.Sum256(buf.Bytes())
	sidecar = fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)
	return tarball, sidecar
}

func TestUpgradeInstallsBehind(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.terminal]
  [[packages.terminal.download]]
  site = "tackle.tools"
  tool = "proj"
  version = "latest"
  bin = "proj"
`)
	// proj installed 0.1.1; latest 0.1.2; the 0.1.2 asset + sha are scripted so Apply succeeds.
	asset, sum := fakeTarball(t, "proj", "0.1.2-binary-bytes")
	restore := stubContext(t, dir,
		map[string]string{"proj version": "proj 0.1.1 (a, d)\n"},
		map[string]string{
			"https://tackle.tools/dl/proj/latest":                                "0.1.2\n",
			"https://tackle.tools/dl/proj/0.1.2/proj_darwin_arm64.tar.gz":        asset,
			"https://tackle.tools/dl/proj/0.1.2/proj_darwin_arm64.tar.gz.sha256": sum,
		})
	defer restore()

	var out, errw bytes.Buffer
	if err := runUpgrade([]string{"-manifest", dir + "/kempt.toml", "-packages", "terminal", "-yes"}, &out, &errw); err != nil {
		t.Fatalf("upgrade: %v (%s)", err, errw.String())
	}
	if !strings.Contains(out.String(), "proj") || !strings.Contains(out.String(), "0.1.2") {
		t.Fatalf("want upgraded proj -> 0.1.2, got %q", out.String())
	}
}

func TestUpgradeUpToDate(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.terminal]
  [[packages.terminal.download]]
  site = "tackle.tools"
  tool = "proj"
  version = "0.1.2"
  bin = "proj"
`)
	restore := stubContext(t, dir, map[string]string{
		"proj version": "proj 0.1.2 (a, d)\n",
	}, map[string]string{})
	defer restore()

	var out, errw bytes.Buffer
	if err := runUpgrade([]string{"-manifest", dir + "/kempt.toml", "-packages", "terminal", "-yes"}, &out, &errw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "everything up to date") {
		t.Fatalf("want up to date message, got: %q", out.String())
	}
}

func TestUpgradeFiltersToNamedTool(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.terminal]
  [[packages.terminal.download]]
  site = "tackle.tools"
  tool = "proj"
  version = "latest"
  bin = "proj"
  [[packages.terminal.download]]
  site = "tackle.tools"
  tool = "other"
  version = "latest"
  bin = "other"
`)
	projAsset, projSum := fakeTarball(t, "proj", "0.1.2-binary-bytes")
	restore := stubContext(t, dir,
		map[string]string{
			"proj version":  "proj 0.1.1 (a, d)\n",
			"other version": "other 2.0.0 (a, d)\n",
		},
		map[string]string{
			"https://tackle.tools/dl/proj/latest":                                "0.1.2\n",
			"https://tackle.tools/dl/proj/0.1.2/proj_darwin_arm64.tar.gz":        projAsset,
			"https://tackle.tools/dl/proj/0.1.2/proj_darwin_arm64.tar.gz.sha256": projSum,
			"https://tackle.tools/dl/other/latest":                               "3.0.0\n",
		})
	defer restore()

	var out, errw bytes.Buffer
	if err := runUpgrade([]string{"-manifest", dir + "/kempt.toml", "-packages", "terminal", "-yes", "proj"}, &out, &errw); err != nil {
		t.Fatalf("upgrade: %v (%s)", err, errw.String())
	}
	if strings.Contains(out.String(), "other") {
		t.Fatalf("upgrade should not have touched other: %q", out.String())
	}
	if !strings.Contains(out.String(), "upgraded proj to 0.1.2") {
		t.Fatalf("want upgraded proj, got %q", out.String())
	}
}

// TestUpgradeSkipsUnresolvableTool covers I1: a "latest" tool whose pointer
// can't be resolved must be skipped with a warning, while a second,
// resolvable behind tool is still upgraded — upgrade must not abort.
func TestUpgradeSkipsUnresolvableTool(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.terminal]
  [[packages.terminal.download]]
  site = "tackle.tools"
  tool = "unreachable"
  version = "latest"
  bin = "unreachable"
  [[packages.terminal.download]]
  site = "tackle.tools"
  tool = "proj"
  version = "latest"
  bin = "proj"
`)
	asset, sum := fakeTarball(t, "proj", "0.1.2-binary-bytes")
	restore := stubContext(t, dir,
		map[string]string{
			"unreachable version": "unreachable 0.1.0 (a, d)\n",
			"proj version":        "proj 0.1.1 (a, d)\n",
		},
		map[string]string{
			// "unreachable"'s pointer is intentionally not scripted.
			"https://tackle.tools/dl/proj/latest":                                "0.1.2\n",
			"https://tackle.tools/dl/proj/0.1.2/proj_darwin_arm64.tar.gz":        asset,
			"https://tackle.tools/dl/proj/0.1.2/proj_darwin_arm64.tar.gz.sha256": sum,
		})
	defer restore()

	var out, errw bytes.Buffer
	if err := runUpgrade([]string{"-manifest", dir + "/kempt.toml", "-packages", "terminal", "-yes"}, &out, &errw); err != nil {
		t.Fatalf("upgrade must not abort on an unresolvable tool: %v (%s)", err, errw.String())
	}
	got := out.String()
	if !strings.Contains(got, "skipping unreachable") || !strings.Contains(got, "could not resolve latest") {
		t.Fatalf("want a skip warning for unreachable, got: %q", got)
	}
	if !strings.Contains(got, "upgraded proj to 0.1.2") {
		t.Fatalf("want proj still upgraded, got: %q", got)
	}
}

func TestUpgradeDeclineAborts(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.terminal]
  [[packages.terminal.download]]
  site = "tackle.tools"
  tool = "proj"
  version = "latest"
  bin = "proj"
`)
	restore := stubContext(t, dir,
		map[string]string{"proj version": "proj 0.1.1 (a, d)\n"},
		map[string]string{"https://tackle.tools/dl/proj/latest": "0.1.2\n"})
	defer restore()
	setStdin(t, "n\n")

	var out, errw bytes.Buffer
	if err := runUpgrade([]string{"-manifest", dir + "/kempt.toml", "-packages", "terminal"}, &out, &errw); err != nil {
		t.Fatalf("upgrade: %v (%s)", err, errw.String())
	}
	if !strings.Contains(out.String(), "aborted") {
		t.Fatalf("want aborted message, got %q", out.String())
	}
	if strings.Contains(out.String(), "upgraded proj") {
		t.Fatalf("decline should not have upgraded: %q", out.String())
	}
}
