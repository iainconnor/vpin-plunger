// move.go — filesystem move helpers for the executor package.
// moveFile and moveDir attempt os.Rename first (atomic on same device);
// fall back to copy+remove on cross-device rename errors.
package executor

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// moveFile moves a single file from src to dst.
// It creates all parent directories of dst.
// Attempts os.Rename first; falls back to copy+remove if src and dst are on
// different devices (EXDEV on Linux, ERROR_NOT_SAME_DEVICE on Windows).
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("moveFile mkdir %s: %w", dst, err)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return fmt.Errorf("moveFile rename %s → %s: %w", src, dst, err)
	}
	return copyThenRemove(src, dst)
}

// moveDir moves a directory from src to dst.
// Attempts os.Rename first (atomic on same device).
// Falls back to recursive filepath.WalkDir copy + os.RemoveAll on cross-device.
func moveDir(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("moveDir mkdir %s: %w", dst, err)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return fmt.Errorf("moveDir rename %s → %s: %w", src, dst, err)
	}
	// Cross-device: recursive copy then remove source
	if err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return fmt.Errorf("moveDir rel %s: %w", p, relErr)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyThenRemove(p, target)
	}); err != nil {
		return fmt.Errorf("moveDir walk %s: %w", src, err)
	}
	return os.RemoveAll(src)
}

// isCrossDevice reports whether err is a cross-device rename error.
// Unwraps *os.LinkError and checks for syscall.EXDEV.
func isCrossDevice(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

// copyThenRemove copies src to dst using io.Copy then removes src.
// Used as cross-device fallback for single files.
func copyThenRemove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copyThenRemove open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("copyThenRemove create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copyThenRemove copy: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("copyThenRemove close dst: %w", err)
	}
	return os.Remove(src)
}
