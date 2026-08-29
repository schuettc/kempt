package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func init() { engine.RegisterHandler(githubReleaseHandler{}) }

// githubReleaseHandler installs a single binary from a GitHub release asset,
// verified against the release's checksums.txt and landed atomically at
// <Home>/.local/bin/<Bin>.
//
// Inspect is strictly offline: it only checks whether the target binary
// exists. Version drift is the verify primitive's concern, keeping plan cheap
// and network-free.
type githubReleaseHandler struct{}

func (githubReleaseHandler) Kind() string { return "github-release" }

func (githubReleaseHandler) Inspect(ctx *machine.Context, s manifest.Step) (engine.Delta, error) {
	st := s.(manifest.GithubReleaseStep)
	bin := binPath(ctx, st.Bin)

	if _, err := os.Stat(bin); err == nil {
		return engine.Delta{Op: engine.OpNoop, Detail: fmt.Sprintf("github-release %s", st.Bin)}, nil
	} else if !os.IsNotExist(err) {
		return engine.Delta{}, err
	}
	return engine.Delta{
		Op:     engine.OpChange,
		Detail: fmt.Sprintf("install %s from %s (latest)", st.Bin, st.Repo),
	}, nil
}

func (githubReleaseHandler) Apply(ctx *machine.Context, s manifest.Step) error {
	st := s.(manifest.GithubReleaseStep)

	tag, err := ctx.Releases.LatestTag(st.Repo)
	if err != nil {
		return err
	}

	asset := st.Asset
	asset = strings.ReplaceAll(asset, "{os}", ctx.OS)
	asset = strings.ReplaceAll(asset, "{arch}", ctx.Arch)
	asset = strings.ReplaceAll(asset, "{tag}", tag)

	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/", st.Repo, tag)
	assetBytes, err := ctx.Releases.Download(baseURL + asset)
	if err != nil {
		return err
	}
	sumBytes, err := ctx.Releases.Download(baseURL + "checksums.txt")
	if err != nil {
		return err
	}

	want, ok := checksumFor(string(sumBytes), asset)
	if !ok {
		return fmt.Errorf("github-release %s: no checksum line for asset %s", st.Bin, asset)
	}
	got := sha256.Sum256(assetBytes)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("github-release %s: checksum mismatch for %s", st.Bin, asset)
	}

	binBytes, err := extractBinary(assetBytes, asset, st.Bin)
	if err != nil {
		return err
	}

	return stageAndRename(ctx, st.Bin, binBytes)
}

// binPath returns <Home>/.local/bin/<bin>.
func binPath(ctx *machine.Context, bin string) string {
	return filepath.Join(ctx.Home, ".local", "bin", bin)
}

// checksumFor finds the sha256 hex for the named asset in checksums.txt
// content. Lines are "<hex>  <name>" (two-space) but a single space is also
// accepted.
func checksumFor(content, asset string) (string, bool) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[len(fields)-1] == asset {
			return fields[0], true
		}
	}
	return "", false
}

// extractBinary returns the binary bytes. When the asset name ends in .tar.gz
// (or .tgz) it decompresses and finds the tar entry whose base name equals
// bin; otherwise the asset bytes are treated as the raw binary.
func extractBinary(assetBytes []byte, asset, bin string) ([]byte, error) {
	if !strings.HasSuffix(asset, ".tar.gz") && !strings.HasSuffix(asset, ".tgz") {
		return assetBytes, nil
	}
	gz, err := gzip.NewReader(bytes.NewReader(assetBytes))
	if err != nil {
		return nil, fmt.Errorf("github-release %s: gzip: %w", bin, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("github-release %s: tar: %w", bin, err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(hdr.Name) == bin {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("github-release %s: %s not found in %s", bin, bin, asset)
}

// stageAndRename writes the binary to a temp path in the target dir (0755)
// then atomically renames it onto the final path.
func stageAndRename(ctx *machine.Context, bin string, data []byte) error {
	final := binPath(ctx, bin)
	dir := filepath.Dir(final)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	staged := filepath.Join(dir, fmt.Sprintf(".%s.new.%d", bin, os.Getpid()))
	if err := os.WriteFile(staged, data, 0o755); err != nil {
		return err
	}
	// WriteFile respects umask; force mode explicitly.
	if err := os.Chmod(staged, 0o755); err != nil {
		os.Remove(staged)
		return err
	}
	if err := os.Rename(staged, final); err != nil {
		os.Remove(staged)
		return err
	}
	return nil
}
