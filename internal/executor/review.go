// review.go — REVIEW.md append helper for the executor package.
// appendREVIEW writes a timestamped markdown section for all items
// needing human attention (review-routed, ignored, failed moves).
package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// reviewEntry is one row in REVIEW.md. Used for SEND_TO_REVIEW, IGNORE,
// Failed, and FailedTwice outcomes.
type reviewEntry struct {
	Source    string    // original source path
	Dest      string    // intended destination (empty for FailedTwice with no dest)
	Action    string    // action type label (e.g. "SEND_TO_REVIEW", "MOVE_VPX")
	Reason    string    // routing reason or error message; for FailedTwice: both errors
	Timestamp time.Time
}

// appendREVIEW appends a timestamped markdown section to review/REVIEW.md.
// It is a no-op when entries is empty (RESEARCH Pitfall 5: never write empty section).
// The file is created if it does not exist; existing content is preserved (O_APPEND).
func appendREVIEW(reviewDir string, entries []reviewEntry, runTime time.Time) error {
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		return fmt.Errorf("appendREVIEW mkdir %s: %w", reviewDir, err)
	}
	path := filepath.Join(reviewDir, "REVIEW.md")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("appendREVIEW open %s: %w", path, err)
	}
	defer f.Close()

	// escapeMD sanitises a string for use in a Markdown pipe-table cell.
	// Pipes break table structure; newlines corrupt rows.
	escapeMD := func(s string) string {
		s = strings.ReplaceAll(s, "|", `\|`)
		s = strings.ReplaceAll(s, "\r\n", " ")
		s = strings.ReplaceAll(s, "\n", " ")
		s = strings.ReplaceAll(s, "\r", "")
		return s
	}

	fmt.Fprintf(f, "\n## Run %s\n\n", runTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "| Source | Intended Destination | Action | Reason | Timestamp |\n")
	fmt.Fprintf(f, "|--------|---------------------|--------|--------|-----------|\n")
	for _, e := range entries {
		fmt.Fprintf(f, "| %s | %s | %s | %s | %s |\n",
			escapeMD(e.Source), escapeMD(e.Dest), escapeMD(e.Action),
			escapeMD(e.Reason), e.Timestamp.Format(time.RFC3339))
	}
	return nil
}
