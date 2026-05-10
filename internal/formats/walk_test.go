package formats

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestWalk_Directory_ARC07(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}

	var visitedFiles []string
	var visitedDirs []string
	err := Walk(context.Background(), tmp, func(p string, d fs.DirEntry, f fs.File) error {
		if d.IsDir() {
			if f != nil {
				t.Errorf("dir %q: f should be nil but got %T", p, f)
			}
			visitedDirs = append(visitedDirs, p)
			return nil
		}
		if f == nil {
			t.Errorf("file %q: f should be non-nil but was nil", p)
			return nil
		}
		visitedFiles = append(visitedFiles, p)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(visitedFiles)

	// The dir-fs root is "."; relative paths should appear for files.
	foundA, foundB := false, false
	for _, v := range visitedFiles {
		if filepath.Base(v) == "a.txt" {
			foundA = true
		}
		if filepath.Base(v) == "b.txt" {
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Fatalf("expected a.txt and sub/b.txt visited, got %v", visitedFiles)
	}
	var foundSub bool
	for _, v := range visitedDirs {
		if filepath.Base(v) == "sub" {
			foundSub = true
		}
	}
	if !foundSub {
		t.Fatalf("expected sub/ visited as a directory, got dirs=%v", visitedDirs)
	}
}

func TestWalk_Archive_ZIP_ARC07(t *testing.T) {
	if _, err := os.Stat(fixtureDist); err != nil {
		t.Fatalf("dist.zip fixture missing — TestMain in handlers_test.go should have created it: %v", err)
	}
	var files []string
	var readBytes int
	err := Walk(context.Background(), fixtureDist, func(p string, d fs.DirEntry, f fs.File) error {
		if d.IsDir() {
			return nil
		}
		files = append(files, p)
		if filepath.Base(p) == "game.vpx" {
			buf, rerr := io.ReadAll(f)
			if rerr != nil {
				return rerr
			}
			readBytes = len(buf)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk archive: %v", err)
	}

	want := []string{"game.vpx", "game.directb2s", "game.ini", "altsound.csv"}
	for _, w := range want {
		var found bool
		for _, f := range files {
			if filepath.Base(f) == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in archive walk, got %v", w, files)
		}
	}
	if readBytes == 0 {
		t.Fatal("expected non-zero bytes read from game.vpx via Walk callback")
	}
}

func TestWalk_PropagatesCallbackError(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("y"), 0o644)

	sentinel := errors.New("stop walking")
	err := Walk(context.Background(), tmp, func(p string, d fs.DirEntry, f fs.File) error {
		if d.IsDir() {
			return nil
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error to surface, got: %v", err)
	}
}

func TestWalk_SkipDir(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "skipme"), 0o755)
	os.WriteFile(filepath.Join(tmp, "keep.txt"), []byte("k"), 0o644)
	os.WriteFile(filepath.Join(tmp, "skipme", "hidden.txt"), []byte("h"), 0o644)

	var visited []string
	err := Walk(context.Background(), tmp, func(p string, d fs.DirEntry, f fs.File) error {
		if d.IsDir() && filepath.Base(p) == "skipme" {
			return fs.SkipDir
		}
		if !d.IsDir() {
			visited = append(visited, filepath.Base(p))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	for _, v := range visited {
		if v == "hidden.txt" {
			t.Fatalf("SkipDir failed: hidden.txt was visited (visited=%v)", visited)
		}
	}
	var foundKeep bool
	for _, v := range visited {
		if v == "keep.txt" {
			foundKeep = true
		}
	}
	if !foundKeep {
		t.Fatalf("expected keep.txt visited, got %v", visited)
	}
}
