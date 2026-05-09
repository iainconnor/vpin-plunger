// Package formats contains all asset detection and format handling logic.
// Zero dependency on the TUI or UI layer.
// Archive traversal via github.com/mholt/archives exclusively.
package formats

import (
	"context"
	"io/fs"

	"github.com/mholt/archives"
)

// Format is the interface all asset handlers must implement.
// Full interface contract defined during GSD milestone 2 per
// MIGRATION-BRIEF.md Section 9.
type Format interface {
	Detect(ctx context.Context, path string, f fs.File) bool
	Name() string
}

// Walk traverses a root path or archive, calling fn for each file.
// Full implementation in milestone 2.
func Walk(ctx context.Context, root string, fn func(path string, f fs.File) error) error {
	fsys, err := archives.FileSystem(ctx, root, nil)
	if err != nil {
		return err
	}
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		f, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return fn(path, f)
	})
}
