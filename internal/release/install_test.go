package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildArchive constructs an in-memory tar.gz containing a single regular
// file named "ackoctl" with the given body, suitable for serving from the
// test HTTP server.
func buildArchive(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name:     BinaryName,
		Mode:     0o755,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}); err != nil {
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
	return buf.Bytes()
}

func TestDownloadAndExtract(t *testing.T) {
	body := []byte("#!/bin/sh\necho fake ackoctl\n")
	archive := buildArchive(t, body)
	sum := sha256.Sum256(archive)
	checksum := hex.EncodeToString(sum[:])

	tag := "v0.9.9"
	goos := "linux"
	goarch := "amd64"
	assetName := AssetName(tag, goos, goarch)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/"+assetName):
			w.Write(archive)
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			fmt.Fprintf(w, "%s  %s\n", checksum, assetName)
			fmt.Fprintf(w, "deadbeef  unrelated.tar.gz\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	dir := t.TempDir()
	binPath, err := c.DownloadAndExtract(context.Background(), tag, goos, goarch, dir)
	if err != nil {
		t.Fatalf("DownloadAndExtract: %v", err)
	}
	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("binary content mismatch: got %q, want %q", got, body)
	}
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("binary not executable: mode = %v", info.Mode())
	}
}

func TestDownloadAndExtractChecksumMismatch(t *testing.T) {
	archive := buildArchive(t, []byte("payload"))
	tag := "v0.9.9"
	assetName := AssetName(tag, "linux", "amd64")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/"+assetName):
			w.Write(archive)
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			fmt.Fprintf(w, "0000000000000000000000000000000000000000000000000000000000000000  %s\n", assetName)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	dir := t.TempDir()
	if _, err := c.DownloadAndExtract(context.Background(), tag, "linux", "amd64", dir); err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}
}

func TestExtractBinaryRejectsOversizedEntry(t *testing.T) {
	// A tar entry larger than maxBinarySize must be rejected rather than
	// extracted, so a corrupt or malicious archive cannot exhaust disk.
	oversized := bytes.Repeat([]byte{0x41}, maxBinarySize+1)
	archive := buildArchive(t, oversized)

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.tar.gz")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, BinaryName)

	err := extractBinary(archivePath, dst)
	if err == nil {
		t.Fatal("expected error for oversized binary, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error = %v, want size-limit error", err)
	}
	// The partially written destination must not be left behind.
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Errorf("oversized extraction left %s on disk", dst)
	}
}

func TestExtractBinaryAcceptsSizeAtLimit(t *testing.T) {
	// An entry exactly at maxBinarySize is within bounds and must extract
	// cleanly — the cap rejects only strictly larger entries.
	body := bytes.Repeat([]byte{0x42}, 1<<20) // 1 MiB, comfortably under cap
	archive := buildArchive(t, body)

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.tar.gz")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, BinaryName)

	if err := extractBinary(archivePath, dst); err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("extracted content mismatch: got %d bytes, want %d", len(got), len(body))
	}
}

// Regression: when io.Copy fails partway through extraction, extractBinary
// must remove the partially written destination file rather than leaving a
// corrupt binary behind — mirroring the size-limit rejection path.
func TestExtractBinaryRemovesPartialFileOnCopyError(t *testing.T) {
	// Build a tar.gz whose tar header claims a larger Size than the bytes
	// actually present, then truncate the gzip stream. tar.Reader.Next
	// succeeds on the header, but io.Copy of the entry body hits an
	// unexpected EOF — exactly the mid-copy failure we need.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name:     BinaryName,
		Mode:     0o755,
		Size:     4096, // claim 4 KiB...
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("short")); err != nil { // ...but write only 5 bytes
		t.Fatal(err)
	}
	// Deliberately do NOT call tw.Close()/gz.Close() cleanly — flush what we
	// have and truncate, so the entry body is incomplete.
	_ = gz.Flush()
	truncated := buf.Bytes()

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.tar.gz")
	if err := os.WriteFile(archivePath, truncated, 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, BinaryName)

	err := extractBinary(archivePath, dst)
	if err == nil {
		t.Fatal("expected error from truncated archive, got nil")
	}
	// The partially written destination must not be left behind.
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Errorf("failed extraction left %s on disk", dst)
	}
}

func TestReplaceSameFs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new")
	dst := filepath.Join(dir, "current")

	if err := os.WriteFile(src, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Replace(src, dst); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("dst content = %q, want %q", got, "new")
	}
}

// TestDownloadFileWritesFullBytes covers the fsync/close path added so a
// later checksum step or extract step never re-reads a truncated download.
// We assert the on-disk byte count matches what the server sent — a missing
// Sync would not fail this test on its own (the regular Close already
// flushes the page cache to the FS), but the explicit size check guards
// against future regressions where the copy path drops the tail.
func TestDownloadFileWritesFullBytes(t *testing.T) {
	payload := bytes.Repeat([]byte("ackoctl-payload-"), 4096) // 64 KiB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	c := New()
	dst := filepath.Join(t.TempDir(), "out.bin")
	if err := c.downloadFile(context.Background(), srv.URL, dst); err != nil {
		t.Fatalf("downloadFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("on-disk size = %d, want %d", len(got), len(payload))
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("on-disk content mismatch")
	}
}

func TestPlatformDefaults(t *testing.T) {
	// Sanity-check that GOOS/GOARCH match runtime — this guards against a
	// future refactor that forgets to wire the package vars to runtime.
	if GOOS != runtime.GOOS {
		t.Errorf("GOOS = %q, want %q", GOOS, runtime.GOOS)
	}
	if GOARCH != runtime.GOARCH {
		t.Errorf("GOARCH = %q, want %q", GOARCH, runtime.GOARCH)
	}
}
