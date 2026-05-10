// download.go - HTTP xlsx download with atomic write + cache staleness check.
// Both Download and IsStale are pure (no Catalog receiver) so tests construct
// arbitrary URL/path combinations; httptest.NewServer integration in catalog_test.go.
package catalog

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// downloadTimeout caps how long a catalog download may take. Two minutes is
// generous for a spreadsheet but prevents an indefinitely blocked tea.Cmd.
const downloadTimeout = 2 * time.Minute

// Download fetches the xlsx file at url and writes it atomically to destPath.
// An atomic write (temp file in the same directory + rename) ensures no partial
// file is left if the download or copy fails. The response body is limited to
// 50 MB to guard against oversized or malicious responses.
func Download(url, destPath string) error {
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	// Limit response to 50 MB to prevent memory exhaustion (RESEARCH.md security domain).
	body := io.LimitReader(resp.Body, 50*1024*1024)

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "catalog-*.xlsx.tmp")
	if err != nil {
		return fmt.Errorf("tempfile: %w", err)
	}
	tmpPath := tmp.Name()
	// Clean up temp file on failure; no-op when os.Rename succeeds (file is gone).
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := io.Copy(tmp, body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("copy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}

	// Atomic rename: src and dst are in the same directory (guaranteed by
	// os.CreateTemp call above), so os.Rename is atomic on most filesystems.
	return os.Rename(tmpPath, destPath)
}

// IsStale reports whether the file at path is missing or older than threshold.
// A missing file is treated as stale (returns true, nil) rather than an error,
// so callers can use the return value directly to trigger a download.
func IsStale(path string, threshold time.Duration) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil // missing = stale, no error
	}
	if err != nil {
		return false, err
	}
	return time.Since(info.ModTime()) > threshold, nil
}
