// walk.go — directory + archive traversal for the formats package.
// Walk delegates to github.com/mholt/archives's FileSystem helper, which
// transparently presents either a real directory or the contents of an
// archive file (ZIP/7z/RAR/tar/etc.) as an fs.FS.
package formats

import (
	"context"
	"io/fs"

	"github.com/mholt/archives"
)

// Walk traverses root (a directory path or an archive file path) and calls
// fn for every entry encountered. The fs.DirEntry parameter lets callers
// inspect d.IsDir() before opening the file — this is required by Phase 4's
// bundle directory pre-pass.
//
// For directory entries (d.IsDir() == true), f is nil and Walk does not open
// anything. Callers must not read from f when d.IsDir() is true.
//
// For file entries (d.IsDir() == false), f is an open fs.File that Walk
// closes after fn returns.
//
// Returning fs.SkipDir from fn skips the directory's remaining children
// (same semantics as fs.WalkDir).
func Walk(ctx context.Context, root string, fn func(path string, d fs.DirEntry, f fs.File) error) error {
	fsys, err := archives.FileSystem(ctx, root, nil)
	if err != nil {
		return err
	}
	return fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return fn(p, d, nil)
		}
		f, oerr := fsys.Open(p)
		if oerr != nil {
			return oerr
		}
		defer f.Close()
		return fn(p, d, f)
	})
}
