package release

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// BinaryName is the file shipped inside the release archive.
const BinaryName = "ackoctl"

// downloadTimeout caps a single release-asset download. Generous compared to
// internal/client's 30s API timeout because release archives are larger, but
// still bounded so a stalled connection cannot hang the upgrade forever.
const downloadTimeout = 5 * time.Minute

// maxBinarySize caps how many bytes extractBinary will copy out of a tar
// entry. 200 MiB is comfortably above any realistic ackoctl binary while
// still bounding a maliciously crafted or corrupt archive so extraction
// cannot exhaust disk.
const maxBinarySize = 200 << 20 // 200 MiB

// DownloadAndExtract pulls the archive for tag/goos/goarch, verifies its
// sha256 against the matching checksums.txt entry, untars the embedded
// `ackoctl` binary into dstDir, and returns the on-disk path of the
// extracted binary. The caller is expected to swap it into place via Replace.
//
// dstDir is created if it does not exist. The function does NOT clean up
// the extracted binary on success — that's the caller's responsibility once
// Replace has moved it into its final location.
func (c *Client) DownloadAndExtract(ctx context.Context, tag, goos, goarch, dstDir string) (string, error) {
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return "", fmt.Errorf("create download dir: %w", err)
	}

	assetName := AssetName(tag, goos, goarch)
	archivePath := filepath.Join(dstDir, assetName)
	if err := c.downloadFile(ctx, c.AssetURL(tag, goos, goarch), archivePath); err != nil {
		return "", err
	}

	// Verify against checksums.txt. Missing checksum entry is treated as an
	// error rather than a warning — for self-upgrade we cannot afford the
	// soft fallback that install.sh allows for diagnostic curl pipes.
	checksumPath := filepath.Join(dstDir, "checksums.txt")
	if err := c.downloadFile(ctx, c.ChecksumsURL(tag), checksumPath); err != nil {
		return "", fmt.Errorf("download checksums.txt: %w", err)
	}
	if err := verifyChecksum(archivePath, checksumPath, assetName); err != nil {
		return "", err
	}

	binPath := filepath.Join(dstDir, BinaryName)
	if err := extractBinary(archivePath, binPath); err != nil {
		return "", err
	}
	if err := os.Chmod(binPath, 0o755); err != nil {
		return "", fmt.Errorf("chmod extracted binary: %w", err)
	}
	return binPath, nil
}

// Replace atomically moves newBinary into the same path as dst (the running
// binary). On the same filesystem this is a single rename(2) — the running
// process keeps executing the old inode and the next invocation picks up
// the new file. If the destination directory is not writable, the caller
// receives a typed error so the CLI can prompt for sudo or suggest a
// different BIN_DIR.
//
// EXDEV (cross-device link) is handled by copying instead of renaming;
// goreleaser-built tarballs and the on-disk binary can end up on different
// mounts when /usr/local/bin is a tmpfs overlay (kind, some CI images).
func Replace(newBinary, dst string) error {
	// Same-fs rename works even when dst is currently executing.
	if err := os.Rename(newBinary, dst); err == nil {
		return nil
	}
	// Rename failed (EXDEV, EACCES on the dest, etc). Fall through to a
	// copy-then-rename within the destination directory.
	return copyReplace(newBinary, dst)
}

// copyReplace implements the cross-device fallback for Replace: copy
// newBinary into a temp file in dst's directory, fsync it, then atomically
// rename it over dst. Split out from Replace so the copy path (which the
// same-fs rename normally short-circuits) is directly exercisable in tests
// without needing two real filesystems.
func copyReplace(newBinary, dst string) error {
	src, err := os.Open(newBinary)
	if err != nil {
		return fmt.Errorf("open extracted binary: %w", err)
	}
	defer src.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".ackoctl-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", filepath.Dir(dst), err)
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("copy binary: %w", err)
	}
	// Flush the copied bytes to stable storage before the rename. Without
	// this, a power loss between rename and the next invocation can leave a
	// truncated ackoctl in place — and unlike a truncated download (which the
	// next run simply re-fetches), a truncated *installed* binary breaks the
	// CLI entirely. The download path already fsyncs for the same reason; this
	// keeps the copy fallback (hit on EXDEV: kind, tmpfs overlays) consistent.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename %s -> %s: %w", tmpName, dst, err)
	}
	return nil
}

// CurrentExecutable returns the absolute path of the running ackoctl
// binary, resolving any symlinks. This is what `ackoctl upgrade` writes to.
func CurrentExecutable() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate running binary: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		// Fall back to the unresolved path — better than failing the upgrade
		// outright on a system where the symlink target is unreachable.
		return p, nil
	}
	return resolved, nil
}

// Platform reports the OS/arch pair used to pick a release asset. Centralised
// here so tests can override via package vars without poking runtime.
var (
	GOOS   = runtime.GOOS
	GOARCH = runtime.GOARCH
)

func (c *Client) downloadFile(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	// Use a dedicated client with an explicit timeout for downloads — we want
	// redirects followed here (the default policy), unlike the Client.HTTP used
	// for LatestTag, but we must not inherit http.DefaultClient's lack of a
	// timeout.
	httpClient := &http.Client{Timeout: downloadTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		// Close (best effort) and remove the partial file so a later
		// checksum step does not see a truncated download.
		f.Close()
		os.Remove(dst)
		return fmt.Errorf("write %s: %w", dst, err)
	}
	// Force the kernel to flush the page cache to disk before the file is
	// handed off to checksum verification or extraction. Without Sync, a
	// power loss between download and the next step could leave a partial
	// file on disk that the next ackoctl invocation re-reads as if it were
	// complete. Close alone does not guarantee a flush to stable storage.
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(dst)
		return fmt.Errorf("sync %s: %w", dst, err)
	}
	// Close explicitly and surface the error — a deferred close swallows
	// delayed-write / out-of-space failures, which would otherwise resurface
	// as a misleading "checksum mismatch".
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dst, err)
	}
	return nil
}

func verifyChecksum(archivePath, checksumPath, assetName string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("reopen archive: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	actual := hex.EncodeToString(h.Sum(nil))

	expected, err := lookupChecksum(checksumPath, assetName)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("checksum mismatch for %s: got %s, expected %s", assetName, actual, expected)
	}
	return nil
}

func lookupChecksum(path, assetName string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open checksums.txt: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// goreleaser produces lines of the form: "<sha256>  <filename>".
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == assetName {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan checksums.txt: %w", err)
	}
	return "", fmt.Errorf("no checksum entry for %s in checksums.txt", assetName)
}

func extractBinary(archivePath, dst string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gunzip archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("archive %s did not contain %s", archivePath, BinaryName)
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != BinaryName {
			continue
		}
		out, err := os.Create(dst)
		if err != nil {
			return fmt.Errorf("create extracted binary: %w", err)
		}
		// Cap extraction at maxBinarySize. LimitReader yields at most
		// maxBinarySize+1 bytes; if the copy reaches exactly that, the entry
		// is larger than the cap and we reject it rather than writing on.
		n, err := io.Copy(out, io.LimitReader(tr, maxBinarySize+1))
		if err != nil {
			out.Close()
			os.Remove(dst)
			return fmt.Errorf("write extracted binary: %w", err)
		}
		if n > maxBinarySize {
			out.Close()
			os.Remove(dst)
			return fmt.Errorf("extracted binary exceeds %d byte limit", maxBinarySize)
		}
		return out.Close()
	}
}
