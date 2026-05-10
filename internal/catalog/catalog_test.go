package catalog

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	excelize "github.com/xuri/excelize/v2"
)

const fixturePath = "testdata/catalog_fixture.xlsx"

func TestMain(m *testing.M) {
	if err := buildFixture(fixturePath); err != nil {
		panic("buildFixture: " + err.Error())
	}
	code := m.Run()
	// Leave the fixture in place for inspection on failure; CI cleans it.
	os.Exit(code)
}

// buildFixture writes a synthetic xlsx with two data sheets and one non-data
// sheet that lacks a "GameName" header (D-07 skip path).
func buildFixture(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f := excelize.NewFile()
	defer f.Close()

	headers := []string{
		"GameName", "Manufact", "GameYear", "GameType",
		"MasterID", "IPDBNum", "DesignedBy", "Decade",
		"Tier", "Notes", "VPW Version Link", "VPS Link", "WebLinkURL",
	}

	type row struct {
		game, manuf         string
		year                any // string or float for the year-fallback test
		master, ipdb        string
	}

	sheets := map[string][]row{
		"Williams": {
			{"Phoenix (Williams, 1978)", "Williams", "1978", "MID-PHX", "1234"},
			{"Firepower (Williams, 1980)", "Williams", "1980", "MID-FP", "1235"},
			{"Flash (Williams, 1979)", "Williams", "1979", "MID-FLASH", "1236"},
			{"Space Shuttle (Williams, 1984)", "Williams", "1984.0", "MID-SS", "1237"}, // float year
			{"Grand Prix (Williams, 1976)", "Williams", "1976", "MID-GP", "1238"},
		},
		"Bally": {
			{"Addams Family, The (Bally, 1992)", "Bally", "1992", "MID-AF", "2001"},
			{"Twilight Zone (Bally, 1993)", "Bally", "1993", "MID-TZ", "2002"},
		},
	}

	for sheet, rows := range sheets {
		f.NewSheet(sheet)
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, cell, h)
		}
		for ri, r := range rows {
			rowNum := ri + 2
			set := func(col int, v any) {
				cell, _ := excelize.CoordinatesToCellName(col, rowNum)
				_ = f.SetCellValue(sheet, cell, v)
			}
			set(1, r.game)
			set(2, r.manuf)
			set(3, r.year)
			set(5, r.master) // MasterID at column 5
			set(6, r.ipdb)   // IPDBNum at column 6
		}
	}

	// Non-data sheet: lacks GameName header. Must be skipped per D-07.
	f.NewSheet("About")
	_ = f.SetCellValue("About", "A1", "This sheet has no GameName header")

	// Default Sheet1 must be removed so it doesn't pollute the entry slice
	// (it has no GameName header, but skipping is per-sheet — keep removal
	// for tidiness).
	_ = f.DeleteSheet("Sheet1")

	return f.SaveAs(path)
}

func defaultTestConfig(t *testing.T, cachePath, sheetURL string) *Config {
	t.Helper()
	return &Config{
		CachePath:       cachePath,
		SheetURL:        sheetURL,
		Staleness:       DefaultStaleness,
		YearWindow:      DefaultYearWindow,
		TrailingArticle: true,
	}
}

// ---------------------------------------------------------------------------
// ParseXLSX tests
// ---------------------------------------------------------------------------

func TestParseXLSX_SkipsNonDataSheet(t *testing.T) {
	f, err := os.Open(fixturePath)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	entries, err := ParseXLSX(f, true)
	if err != nil {
		t.Fatalf("ParseXLSX: %v", err)
	}
	// 5 Williams + 2 Bally = 7. About sheet must be skipped (D-07).
	if len(entries) != 7 {
		t.Fatalf("entries = %d, want 7 (Williams=5 + Bally=2; About must skip)", len(entries))
	}
}

func TestParseXLSX_YearFloatFallback(t *testing.T) {
	f, err := os.Open(fixturePath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	entries, err := ParseXLSX(f, true)
	if err != nil {
		t.Fatalf("ParseXLSX: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.MasterID == "MID-SS" {
			if e.Year != 1984 {
				t.Fatalf("Space Shuttle Year = %d, want 1984", e.Year)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("MID-SS entry not found in fixture")
	}
}

// ---------------------------------------------------------------------------
// IsStale tests
// ---------------------------------------------------------------------------

func TestIsStale_Missing(t *testing.T) {
	tmp := t.TempDir()
	stale, err := IsStale(filepath.Join(tmp, "does-not-exist.xlsx"), 7*24*time.Hour)
	if err != nil {
		t.Fatalf("IsStale: %v", err)
	}
	if !stale {
		t.Fatal("expected stale=true for missing file")
	}
}

func TestIsStale_Fresh(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "fresh.xlsx")
	if err := os.WriteFile(path, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err := IsStale(path, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("IsStale: %v", err)
	}
	if stale {
		t.Fatal("expected stale=false for just-written file")
	}
}

// ---------------------------------------------------------------------------
// SheetEntry.CanonicalFilename test (CAT-03)
// ---------------------------------------------------------------------------

func TestSheetEntry_CanonicalFilename(t *testing.T) {
	e := SheetEntry{Name: "The Addams Family", Manufacturer: "Bally", Year: 1992}
	if got := e.CanonicalFilename(true); got != "Addams Family, The (Bally, 1992)" {
		t.Fatalf("trailingArticle=true: got %q", got)
	}
	if got := e.CanonicalFilename(false); got != "The Addams Family (Bally, 1992)" {
		t.Fatalf("trailingArticle=false: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// HTTP download tests (CAT-01) using httptest.NewServer (D-18)
// ---------------------------------------------------------------------------

func TestLoad_Download(t *testing.T) {
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	cfg := defaultTestConfig(t, filepath.Join(tmp, "cache", "catalog.xlsx"), srv.URL+"/export?format=xlsx")
	c := New(cfg)
	if err := c.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(c.Entries()); got != 7 {
		t.Fatalf("Entries() = %d, want 7", got)
	}
	// Cache file must exist post-Load.
	if _, err := os.Stat(cfg.CachePath); err != nil {
		t.Fatalf("expected cache at %s: %v", cfg.CachePath, err)
	}
}

func TestLoad_ReusesFreshCache(t *testing.T) {
	// Pre-populate cache with the fixture; server returns garbage. Load
	// should NOT hit the server because the cache is fresh.
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "catalog.xlsx")
	if err := os.WriteFile(cachePath, fixtureBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultTestConfig(t, cachePath, srv.URL)
	c := New(cfg)
	if err := c.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if hit {
		t.Fatal("server was hit despite fresh cache")
	}
	if got := len(c.Entries()); got != 7 {
		t.Fatalf("Entries() = %d, want 7", got)
	}
}

func TestLoad_ParseOnce(t *testing.T) {
	// CAT-09: a second Load() call repopulates entries cleanly (idempotent).
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	cfg := defaultTestConfig(t, filepath.Join(tmp, "cache.xlsx"), srv.URL)
	c := New(cfg)
	if err := c.Load(); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	first := len(c.Entries())
	if err := c.Load(); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if got := len(c.Entries()); got != first {
		t.Fatalf("entry count changed across Load calls: %d -> %d", first, got)
	}
}

// ---------------------------------------------------------------------------
// Matching tests (CAT-05, CAT-06, CAT-07)
// ---------------------------------------------------------------------------

func loadedCatalog(t *testing.T, trailingArticle bool) *Catalog {
	t.Helper()
	// Build a Catalog directly from parsed entries — bypassing download.
	f, err := os.Open(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	entries, err := ParseXLSX(f, trailingArticle)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		YearWindow:      DefaultYearWindow,
		TrailingArticle: trailingArticle,
		Staleness:       DefaultStaleness,
	}
	c := &Catalog{cfg: cfg, entries: entries}
	return c
}

func TestFindMatch_PathA_SameEra(t *testing.T) {
	c := loadedCatalog(t, true)
	results, err := c.FindMatch("Phoenix (Williams, 1978)", 5)
	if err != nil {
		t.Fatalf("FindMatch error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("FindMatch returned no candidates")
	}
	top := results[0]
	if top.Entry.MasterID != "MID-PHX" {
		t.Fatalf("top MasterID = %q, want MID-PHX", top.Entry.MasterID)
	}
	if top.Confidence < ThresholdInteractive {
		t.Fatalf("top confidence = %d, want >= %d", top.Confidence, ThresholdInteractive)
	}
}

func TestFindMatch_PathB_FullCatalogFallback(t *testing.T) {
	c := loadedCatalog(t, true)
	// Stem with no signal at all: forces Path B.
	results, err := c.FindMatch("Phoenix v600", 3)
	if err != nil {
		t.Fatalf("FindMatch error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Path B returned no candidates")
	}
	// Top result should still be Phoenix.
	if results[0].Entry.MasterID != "MID-PHX" {
		t.Fatalf("Path B top MasterID = %q, want MID-PHX", results[0].Entry.MasterID)
	}
}

func TestBestMatch_AboveThreshold(t *testing.T) {
	c := loadedCatalog(t, true)
	m, err := c.BestMatch("Phoenix (Williams, 1978)")
	if err != nil {
		t.Fatalf("BestMatch error: %v", err)
	}
	if m == nil {
		t.Fatal("BestMatch nil for high-quality input")
	}
	if m.Confidence < ThresholdInteractive {
		t.Fatalf("BestMatch confidence = %d, want >= %d", m.Confidence, ThresholdInteractive)
	}
}

func TestBestMatch_BelowThreshold(t *testing.T) {
	c := loadedCatalog(t, true)
	m, err := c.BestMatch("ZZZ totally unrelated string xyzzy")
	if err != nil {
		t.Fatalf("BestMatch error: %v", err)
	}
	if m != nil {
		t.Fatalf("BestMatch = %+v, want nil for unmatched query", m)
	}
}

func TestForceMatch_MasterID(t *testing.T) {
	c := loadedCatalog(t, true)
	m, err := c.ForceMatch("MID-PHX")
	if err != nil {
		t.Fatalf("ForceMatch error: %v", err)
	}
	if m == nil {
		t.Fatal("ForceMatch returned nil for known MasterID")
	}
	if m.Confidence != 100 {
		t.Fatalf("Confidence = %d, want 100", m.Confidence)
	}
	if m.MatchField != "master_id" {
		t.Fatalf("MatchField = %q, want master_id", m.MatchField)
	}
}

func TestForceMatch_IPDBNum(t *testing.T) {
	c := loadedCatalog(t, true)
	m, err := c.ForceMatch("2001")
	if err != nil {
		t.Fatalf("ForceMatch error: %v", err)
	}
	if m == nil {
		t.Fatal("ForceMatch returned nil for known IPDBNum")
	}
	if m.Confidence != 100 {
		t.Fatalf("Confidence = %d, want 100", m.Confidence)
	}
	if m.MatchField != "ipdb_num" {
		t.Fatalf("MatchField = %q, want ipdb_num", m.MatchField)
	}
}

func TestForceMatch_Unknown(t *testing.T) {
	c := loadedCatalog(t, true)
	m, err := c.ForceMatch("NOT-A-REAL-ID")
	if err != nil {
		t.Fatalf("ForceMatch error: %v", err)
	}
	if m != nil {
		t.Fatalf("ForceMatch = %+v, want nil", m)
	}
}

func TestFindMatch_ErrNotLoaded(t *testing.T) {
	c := New(&Config{YearWindow: DefaultYearWindow})
	_, err := c.FindMatch("Phoenix", 5)
	if err != ErrNotLoaded {
		t.Fatalf("FindMatch on unloaded catalog: got %v, want ErrNotLoaded", err)
	}
}

func TestBestMatch_ErrNotLoaded(t *testing.T) {
	c := New(&Config{YearWindow: DefaultYearWindow})
	_, err := c.BestMatch("Phoenix")
	if err != ErrNotLoaded {
		t.Fatalf("BestMatch on unloaded catalog: got %v, want ErrNotLoaded", err)
	}
}

func TestForceMatch_ErrNotLoaded(t *testing.T) {
	c := New(&Config{YearWindow: DefaultYearWindow})
	_, err := c.ForceMatch("MID-PHX")
	if err != ErrNotLoaded {
		t.Fatalf("ForceMatch on unloaded catalog: got %v, want ErrNotLoaded", err)
	}
}
