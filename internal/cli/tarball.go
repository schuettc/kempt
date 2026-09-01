package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// isTarballURL reports whether ref points at a gzipped tarball to fetch and
// extract (e.g. a GitHub codeload archive) rather than a git repo to clone.
func isTarballURL(ref string) bool {
	return isManifestURL(ref) && (strings.HasSuffix(ref, ".tar.gz") || strings.HasSuffix(ref, ".tgz"))
}

// openTarballStream opens the tarball body for reading. It is a package var so
// tests can supply an in-memory archive without touching the network.
var openTarballStream = func(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return resp.Body, nil
}

// fetchTarball fetches url and extracts it into dir. It strips the single
// leading path component every member shares — GitHub codeload tarballs wrap
// the tree in a `<repo>-<ref>/` directory — so files land directly under dir.
func fetchTarball(url, dir string) error {
	body, err := openTarballStream(url)
	if err != nil {
		return err
	}
	defer body.Close()
	return extractTarballStripped(body, dir)
}

// extractTarballStripped extracts a gzipped tar stream into dir, stripping the
// first path component of each member. It is path-traversal safe (a member that
// would escape dir is rejected) and refuses symlink members, which a config
// tree does not need and which are an escape vector.
func extractTarballStripped(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		rel := stripLeadingComponent(hdr.Name)
		if rel == "" {
			continue
		}
		target := filepath.Join(root, rel)
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return fmt.Errorf("tar member escapes destination: %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, io.LimitReader(tr, 512<<20)); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("tar member %q is a link; refusing to extract config tree with links", hdr.Name)
		default:
			// skip devices, fifos, etc.
		}
	}
	return nil
}

// stripLeadingComponent drops the first slash-separated component of an archive
// member name. A name with no separator (the wrapper dir entry itself) yields "".
func stripLeadingComponent(name string) string {
	name = strings.TrimPrefix(name, "./")
	i := strings.IndexByte(name, '/')
	if i < 0 {
		return ""
	}
	return strings.TrimPrefix(name[i+1:], "/")
}
