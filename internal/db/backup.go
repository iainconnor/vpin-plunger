package db

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupBeforeFirstWrite creates one backup of dbPath per session.
// Subsequent calls in the same session are no-ops (d.backed is set on first call).
// backupDir is created if it does not exist.
// Filename format: YYYYMMDDHHMMSS_PUPDatabase.db (timestamp-first for chronological sort).
// After backup, pruneBackups(backupDir, 5, 5) trims old backups (EXE-06).
func (d *DB) BackupBeforeFirstWrite(dbPath, backupDir string) error {
	if d.backed {
		return nil
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("backup mkdir %s: %w", backupDir, err)
	}
	ts := time.Now().Format("20060102150405")
	dst := filepath.Join(backupDir, ts+"_PUPDatabase.db")
	if err := copyFile(dbPath, dst); err != nil {
		return fmt.Errorf("backup copy: %w", err)
	}
	d.backed = true
	return pruneBackups(backupDir, 5, 5)
}

// pruneBackups removes old backup files to stay within maxPerDay backups per
// calendar day and maxDays calendar days total.
// Files must match the suffix "_PUPDatabase.db" and have an 8-char YYYYMMDD prefix.
// os.ReadDir returns entries sorted by name; timestamp-first names sort
// chronologically — no explicit time parse needed.
func pruneBackups(backupDir string, maxPerDay, maxDays int) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("pruneBackups readdir %s: %w", backupDir, err)
	}

	// Filter to backup files only.
	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_PUPDatabase.db") {
			backups = append(backups, e.Name())
		}
	}
	// backups is already sorted lexicographically (= chronologically) by os.ReadDir.

	// Group by YYYYMMDD prefix (first 8 chars).
	type dayGroup struct {
		day   string
		files []string
	}
	var days []dayGroup
	for _, name := range backups {
		if len(name) < 8 {
			continue
		}
		day := name[:8]
		if len(days) == 0 || days[len(days)-1].day != day {
			days = append(days, dayGroup{day: day})
		}
		days[len(days)-1].files = append(days[len(days)-1].files, name)
	}

	// Collect files to prune.
	var toDelete []string

	// Per-day limit: keep last maxPerDay in each day.
	for i := range days {
		if len(days[i].files) > maxPerDay {
			// Delete the oldest (earliest in sorted slice).
			excess := days[i].files[:len(days[i].files)-maxPerDay]
			toDelete = append(toDelete, excess...)
			days[i].files = days[i].files[len(days[i].files)-maxPerDay:]
		}
	}

	// Days limit: keep last maxDays calendar days.
	if len(days) > maxDays {
		for _, dg := range days[:len(days)-maxDays] {
			toDelete = append(toDelete, dg.files...)
		}
	}

	// Sort and deduplicate toDelete before removing.
	sort.Strings(toDelete)
	seen := make(map[string]bool)
	for _, name := range toDelete {
		if seen[name] {
			continue
		}
		seen[name] = true
		if err := os.Remove(filepath.Join(backupDir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("pruneBackups remove %s: %w", name, err)
		}
	}
	return nil
}

// copyFile copies the file at src to dst, creating dst if it does not exist.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copyFile open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("copyFile create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copyFile copy: %w", err)
	}
	return out.Close()
}
