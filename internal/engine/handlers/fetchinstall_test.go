package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bareHexSums returns a sums file that is a bare hex string (64 hex chars +
// optional trailing newline) with no filename component — the ".sha256 sidecar"
// format used by some simple release tooling.
func bareHexSums(data []byte) []byte {
	sum := sha256.Sum256(data)
	return []byte(hex.EncodeToString(sum[:]) + "\n")
}

// dotSlashSums returns a sums file whose filename is prefixed with "./" as
// produced by shasum/sha256sum when run from the directory containing the file.
func dotSlashSums(asset string, data []byte) []byte {
	sum := sha256.Sum256(data)
	return []byte(fmt.Sprintf("%s  ./%s\n", hex.EncodeToString(sum[:]), asset))
}

// ---------- checksumHex unit tests ----------

func TestChecksumHexBareHex(t *testing.T) {
	data := []byte("some-binary-payload")
	sumsFile := bareHexSums(data)

	// checksumHex with a bare-hex file should succeed regardless of assetName.
	got, err := checksumHex(sumsFile, "demo_darwin_arm64.tar.gz")
	if err != nil {
		t.Fatalf("checksumHex bare-hex: unexpected error: %v", err)
	}

	want := hex.EncodeToString(sha256Sum(data))
	if got != want {
		t.Fatalf("checksumHex bare-hex: got %q, want %q", got, want)
	}
}

func TestChecksumHexBareHexNoNewline(t *testing.T) {
	data := []byte("another-payload")
	sum := sha256.Sum256(data)
	sumsFile := []byte(hex.EncodeToString(sum[:]))

	got, err := checksumHex(sumsFile, "tool.tar.gz")
	if err != nil {
		t.Fatalf("checksumHex bare-hex (no newline): unexpected error: %v", err)
	}
	if got != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksumHex bare-hex (no newline): got %q", got)
	}
}

func TestChecksumHexDotSlashPrefix(t *testing.T) {
	assetName := "demo_darwin_arm64.tar.gz"
	data := []byte("darwin arm64 binary")
	sumsFile := dotSlashSums(assetName, data)

	got, err := checksumHex(sumsFile, assetName)
	if err != nil {
		t.Fatalf("checksumHex ./-prefix: unexpected error: %v", err)
	}
	want := hex.EncodeToString(sha256Sum(data))
	if got != want {
		t.Fatalf("checksumHex ./-prefix: got %q, want %q", got, want)
	}
}

func TestChecksumHexDotSlashPrefixMultiLine(t *testing.T) {
	assetName := "demo_darwin_arm64.tar.gz"
	data := []byte("target payload")
	sum := sha256.Sum256(data)

	// Multi-line sums file with ./ prefix on the target entry.
	sumsFile := []byte(strings.Join([]string{
		"aabbccdd  other_linux_amd64.tar.gz",
		fmt.Sprintf("%s  ./%s", hex.EncodeToString(sum[:]), assetName),
		"eeff0011  third_windows_amd64.zip",
		"",
	}, "\n"))

	got, err := checksumHex(sumsFile, assetName)
	if err != nil {
		t.Fatalf("checksumHex ./-prefix multi-line: unexpected error: %v", err)
	}
	if got != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksumHex ./-prefix multi-line: got %q", got)
	}
}

// ---------- verifyExtractInstall end-to-end tests ----------

func TestVerifyExtractInstallBareHexSums(t *testing.T) {
	assetName := "demo_darwin_arm64.tar.gz"
	content := []byte("#!/bin/sh\necho demo-darwin\n")
	tgz := makeTarGz(t, "demo", content)
	sumsFile := bareHexSums(tgz)

	destDir := t.TempDir()
	if err := verifyExtractInstall(assetName, tgz, sumsFile, "demo", destDir); err != nil {
		t.Fatalf("verifyExtractInstall bare-hex: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("binary content mismatch: %q", got)
	}
}

func TestVerifyExtractInstallDotSlashSums(t *testing.T) {
	assetName := "demo_darwin_arm64.tar.gz"
	content := []byte("#!/bin/sh\necho demo-dotslash\n")
	tgz := makeTarGz(t, "demo", content)
	sumsFile := dotSlashSums(assetName, tgz)

	destDir := t.TempDir()
	if err := verifyExtractInstall(assetName, tgz, sumsFile, "demo", destDir); err != nil {
		t.Fatalf("verifyExtractInstall ./-prefix: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("binary content mismatch: %q", got)
	}
}

func TestVerifyExtractInstallBinContextInErrors(t *testing.T) {
	assetName := "demo_linux_amd64.tar.gz"
	tgz := makeTarGz(t, "demo", []byte("real"))

	t.Run("checksum mismatch names bin", func(t *testing.T) {
		badSums := bareHexSums([]byte("wrong-data"))
		err := verifyExtractInstall(assetName, tgz, badSums, "demo", t.TempDir())
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "verify demo") {
			t.Fatalf("error should contain 'verify demo', got: %v", err)
		}
	})

	t.Run("missing checksum line names bin", func(t *testing.T) {
		noMatch := []byte("aabbccdd  other.tar.gz\n")
		err := verifyExtractInstall(assetName, tgz, noMatch, "demo", t.TempDir())
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "verify demo") {
			t.Fatalf("error should contain 'verify demo', got: %v", err)
		}
	})
}

// sha256Sum is a small helper to keep test code readable.
func sha256Sum(data []byte) []byte {
	s := sha256.Sum256(data)
	return s[:]
}
