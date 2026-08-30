// Package selfupdate replaces the running kempt binary with a newer release
// asset, verified against the release's checksums.txt and landed atomically.
//
// The verified-atomic pattern mirrors the github-release handler: download the
// asset + checksums.txt, sha256-verify the asset line, extract the "kempt"
// entry from the tar.gz (or treat the asset as a raw binary), stage beside the
// target as .kempt.new.<pid> (0755), then os.Rename onto the target.
package selfupdate

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

	"github.com/schuettc/kempt/internal/release"
)

// Options configures a self-update.
type Options struct {
	Repo     string // "schuettc/kempt"
	Asset    string // "kempt_{os}_{arch}.tar.gz"
	OS       string
	Arch     string
	Current  string // version.Number(); compared to tag with "v" trimmed
	ExePath  string // resolved target to replace
	Releases release.Releases
}

// Update resolves the latest release tag for o.Repo. When the tag (with a
// leading "v" trimmed) equals o.Current, it is a no-op and returns
// (false, "", nil). Otherwise it downloads and verifies the asset, extracts the
// "kempt" binary, and atomically replaces o.ExePath, returning (true, tag, nil).
//
// On any download, checksum, or extraction failure nothing is written to
// ExePath.
func Update(o Options) (bool, string, error) {
	tag, err := o.Releases.LatestTag(o.Repo)
	if err != nil {
		return false, "", err
	}
	if strings.TrimPrefix(tag, "v") == o.Current {
		return false, "", nil
	}

	asset := o.Asset
	asset = strings.ReplaceAll(asset, "{os}", o.OS)
	asset = strings.ReplaceAll(asset, "{arch}", o.Arch)
	asset = strings.ReplaceAll(asset, "{tag}", tag)

	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/", o.Repo, tag)
	assetBytes, err := o.Releases.Download(baseURL + asset)
	if err != nil {
		return false, "", err
	}
	sumBytes, err := o.Releases.Download(baseURL + "checksums.txt")
	if err != nil {
		return false, "", err
	}

	want, ok := checksumFor(string(sumBytes), asset)
	if !ok {
		return false, "", fmt.Errorf("selfupdate: no checksum line for asset %s", asset)
	}
	got := sha256.Sum256(assetBytes)
	if hex.EncodeToString(got[:]) != strings.ToLower(want) {
		return false, "", fmt.Errorf("selfupdate: checksum mismatch for %s", asset)
	}

	binBytes, err := extractBinary(assetBytes, asset)
	if err != nil {
		return false, "", err
	}

	if err := stageAndRename(o.ExePath, binBytes); err != nil {
		return false, "", err
	}
	return true, tag, nil
}

// checksumFor finds the sha256 hex for the named asset in checksums.txt content.
// Lines are "<hex>  <name>" (two-space) but a single space is also accepted.
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

// extractBinary returns the kempt binary bytes. When the asset name ends in
// .tar.gz (or .tgz) it decompresses and finds the tar entry whose base name is
// "kempt"; otherwise the asset bytes are treated as the raw binary.
func extractBinary(assetBytes []byte, asset string) ([]byte, error) {
	if !strings.HasSuffix(asset, ".tar.gz") && !strings.HasSuffix(asset, ".tgz") {
		return assetBytes, nil
	}
	gz, err := gzip.NewReader(bytes.NewReader(assetBytes))
	if err != nil {
		return nil, fmt.Errorf("selfupdate: gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("selfupdate: tar: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(hdr.Name) == "kempt" {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("selfupdate: kempt not found in %s", asset)
}

// stageAndRename writes data to a temp path beside final (0755) then atomically
// renames it onto final.
func stageAndRename(final string, data []byte) error {
	dir := filepath.Dir(final)
	staged := filepath.Join(dir, fmt.Sprintf(".kempt.new.%d", os.Getpid()))
	if err := os.WriteFile(staged, data, 0o755); err != nil {
		os.Remove(staged)
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
