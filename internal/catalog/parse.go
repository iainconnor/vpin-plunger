// parse.go - excelize-based xlsx parser that returns []SheetEntry.
// ParseXLSX is a pure function accepting io.Reader so tests can pass
// bytes.NewReader without touching the filesystem. Sheets lacking a
// "GameName" header in row 1 are skipped per D-07.
package catalog

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	excelize "github.com/xuri/excelize/v2"

	"github.com/iainconnor/vpin-plunger/internal/naming"
)

// ParseXLSX reads an xlsx file from r, iterates all sheets, and returns the
// combined []SheetEntry from every sheet whose row 1 contains a "GameName"
// header (D-07: skip non-data tabs). trailingArticle is forwarded to the
// naming.ExtractSignal-based field population (reserved for future per-row
// canonicalization; entries store raw values today).
func ParseXLSX(r io.Reader, trailingArticle bool) ([]SheetEntry, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	var entries []SheetEntry
	for _, sheetName := range f.GetSheetList() {
		rows, err := f.Rows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("rows(%s): %w", sheetName, err)
		}
		sheetEntries := parseSheet(rows, trailingArticle)
		// CRITICAL: explicit Close (not defer) so we surface the error per
		// sheet and release the temporary XML file — Pitfall 2 in RESEARCH.md.
		if cerr := rows.Close(); cerr != nil {
			return nil, fmt.Errorf("rows.Close(%s): %w", sheetName, cerr)
		}
		entries = append(entries, sheetEntries...)
	}
	return entries, nil
}

// parseSheet consumes a *excelize.Rows iterator and returns []SheetEntry.
// Row 1 is treated as the header row. If "GameName" is absent from the
// header, the sheet is skipped (D-07 header-only filter). Remaining rows
// are drained so rows.Close() releases the underlying XML file cleanly.
func parseSheet(rows *excelize.Rows, trailingArticle bool) []SheetEntry {
	if !rows.Next() {
		return nil // empty sheet
	}
	headers, _ := rows.Columns()
	colIdx := make(map[string]int, len(headers))
	for i, h := range headers {
		colIdx[strings.TrimSpace(h)] = i
	}
	if _, ok := colIdx["GameName"]; !ok {
		// D-07: skip non-data tabs (header-only filter).
		// Drain remaining rows so rows.Close() releases cleanly.
		for rows.Next() {
		}
		return nil
	}

	// col resolves a named column value from a row, returning "" for missing
	// columns or out-of-bounds row indexes.
	col := func(row []string, name string) string {
		idx, ok := colIdx[name]
		if !ok || idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	var entries []SheetEntry
	for rows.Next() {
		row, _ := rows.Columns()
		if allEmpty(row) {
			continue
		}
		gameName := col(row, "GameName")
		if gameName == "" {
			continue
		}
		// ExtractSignal lifts {Name, Manufacturer, Year} from a structured
		// GameName like "Phoenix (Williams, 1978)". When absent, fall back
		// to the column values.
		sig := naming.ExtractSignal(gameName)
		name := gameName
		manuf := col(row, "Manufact")
		year := parseYear(col(row, "GameYear"))
		if sig != nil {
			name = sig.Name
			if manuf == "" {
				manuf = sig.Manufacturer
			}
			if year == 0 {
				year = sig.Year
			}
		}
		entries = append(entries, SheetEntry{
			GameName:     gameName,
			Name:         name,
			Manufacturer: manuf,
			Year:         year,
			GameType:     col(row, "GameType"),
			MasterID:     col(row, "MasterID"),
			IPDBNum:      col(row, "IPDBNum"),
			DesignedBy:   col(row, "DesignedBy"),
			Decade:       col(row, "Decade"),
			Tier:         col(row, "Tier"),
			Notes:        col(row, "Notes"),
			VPWLink:      col(row, "VPW Version Link"),
			VPSLink:      col(row, "VPS Link"),
			IPDBUrl:      col(row, "WebLinkURL"),
		})
	}
	_ = trailingArticle // reserved for future per-row canonicalization; entries store raw values
	return entries
}

// allEmpty reports whether every cell in row is blank after trimming whitespace.
func allEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// parseYear converts an xlsx cell string to an int year. Excel sometimes
// serializes years as floats (e.g. "1978.0"); the float fallback handles both
// integer and float-shaped strings, matching the Python `int(float(raw))` idiom.
func parseYear(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f)
	}
	return 0
}
