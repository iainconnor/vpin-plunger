// handlers.go — concrete Handler implementations for ZIP, 7z, RAR.
// All extract functions enforce zip-slip protection (any member path that
// resolves outside dest is rejected). RAR errors are translated to actionable
// messages by wrapRARError.
package formats

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"
)

// ZIPHandler handles .zip archives using the archive/zip stdlib (D-08).
type ZIPHandler struct{}

func (ZIPHandler) Name() string { return "ZIP" }

func (ZIPHandler) Detect(_ context.Context, p string, _ fs.File) bool {
	return strings.EqualFold(filepath.Ext(p), ".zip")
}

func (ZIPHandler) Peek(p string) ([]string, error) {
	r, err := zip.OpenReader(p)
	if err != nil {
		return nil, fmt.Errorf("zip peek %s: %w", p, err)
	}
	defer r.Close()
	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, strings.ReplaceAll(f.Name, `\`, `/`))
	}
	return names, nil
}

func (ZIPHandler) Extract(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("zip extract %s: %w", src, err)
	}
	defer r.Close()

	destClean := filepath.Clean(dest) + string(os.PathSeparator)
	for _, f := range r.File {
		name := strings.ReplaceAll(f.Name, `\`, `/`)
		target := filepath.Join(destClean, name)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), destClean) {
			return fmt.Errorf("zip extract %s: path escape rejected: %s", src, name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("zip extract %s: mkdir %s: %w", src, target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("zip extract %s: mkdir parent of %s: %w", src, target, err)
		}
		if err := copyZipMember(f, target); err != nil {
			return fmt.Errorf("zip extract %s: member %s: %w", src, name, err)
		}
	}
	return nil
}

func copyZipMember(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	wf, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err = io.Copy(wf, rc); err != nil {
		wf.Close()
		return err
	}
	return wf.Close() // surface flush/sync errors
}

// SevenZipHandler handles .7z archives using github.com/bodgit/sevenzip (D-09).
type SevenZipHandler struct{}

func (SevenZipHandler) Name() string { return "7z" }

func (SevenZipHandler) Detect(_ context.Context, p string, _ fs.File) bool {
	return strings.EqualFold(filepath.Ext(p), ".7z")
}

func (SevenZipHandler) Peek(p string) ([]string, error) {
	r, err := sevenzip.OpenReader(p)
	if err != nil {
		return nil, fmt.Errorf("7z peek %s: %w", p, err)
	}
	defer r.Close()
	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names, nil
}

func (SevenZipHandler) Extract(src, dest string) error {
	r, err := sevenzip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("7z extract %s: %w", src, err)
	}
	defer r.Close()

	destClean := filepath.Clean(dest) + string(os.PathSeparator)
	for _, f := range r.File {
		target := filepath.Join(destClean, f.Name)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), destClean) {
			return fmt.Errorf("7z extract %s: path escape rejected: %s", src, f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("7z extract %s: mkdir %s: %w", src, target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("7z extract %s: mkdir parent of %s: %w", src, target, err)
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("7z extract %s: open member %s: %w", src, f.Name, err)
		}
		wf, err := os.Create(target)
		if err != nil {
			rc.Close()
			return fmt.Errorf("7z extract %s: create %s: %w", src, target, err)
		}
		_, copyErr := io.Copy(wf, rc)
		rc.Close()
		wf.Close()
		if copyErr != nil {
			return fmt.Errorf("7z extract %s: copy member %s: %w", src, f.Name, copyErr)
		}
	}
	return nil
}

// RARHandler handles .rar archives using github.com/nwaples/rardecode/v2 (D-10).
// Pure-Go only; no shell-out. Solid/encrypted/unsupported archives return
// actionable errors via wrapRARError.
type RARHandler struct{}

func (RARHandler) Name() string { return "RAR" }

func (RARHandler) Detect(_ context.Context, p string, _ fs.File) bool {
	return strings.EqualFold(filepath.Ext(p), ".rar")
}

func (RARHandler) Peek(p string) ([]string, error) {
	files, err := rardecode.List(p)
	if err != nil {
		return nil, wrapRARError(p, "peek", err)
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	return names, nil
}

func (RARHandler) Extract(src, dest string) error {
	r, err := rardecode.OpenReader(src)
	if err != nil {
		return wrapRARError(src, "extract", err)
	}
	defer r.Close()

	destClean := filepath.Clean(dest) + string(os.PathSeparator)
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return wrapRARError(src, "extract", err)
		}
		target := filepath.Join(destClean, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), destClean) {
			return fmt.Errorf("rar extract %s: path escape rejected: %s", src, hdr.Name)
		}
		if hdr.IsDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("rar extract %s: mkdir %s: %w", src, target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("rar extract %s: mkdir parent of %s: %w", src, target, err)
		}
		wf, err := os.Create(target)
		if err != nil {
			return fmt.Errorf("rar extract %s: create %s: %w", src, target, err)
		}
		_, copyErr := io.Copy(wf, r)
		wf.Close()
		if copyErr != nil {
			return wrapRARError(src, "extract member "+hdr.Name, copyErr)
		}
	}
	return nil
}

// wrapRARError translates rardecode sentinel errors into operator-readable
// messages. Matches RESEARCH.md Pattern 7.
func wrapRARError(p, op string, err error) error {
	switch {
	case errors.Is(err, rardecode.ErrSolidOpen):
		return fmt.Errorf("rar %s %s: solid archive not supported for random access", op, p)
	case errors.Is(err, rardecode.ErrArchivedFileEncrypted):
		return fmt.Errorf("rar %s %s: archive is encrypted; provide password to extract", op, p)
	case errors.Is(err, rardecode.ErrUnknownVersion):
		return fmt.Errorf("rar %s %s: unsupported RAR version (RAR5 with AES or newer format)", op, p)
	default:
		return fmt.Errorf("rar %s %s: %w", op, p, err)
	}
}
