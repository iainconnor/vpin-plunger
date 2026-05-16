package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite" // registers driver for test sql.Open calls
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// createSchema creates the Emulators and Games tables in sqldb to match the
// PUPDatabase structure that validateColumns expects.
func createSchema(sqldb *sql.DB) error {
	_, err := sqldb.Exec(`
		CREATE TABLE IF NOT EXISTS Emulators (
			EMUID   INTEGER PRIMARY KEY AUTOINCREMENT,
			EmuName TEXT
		)`)
	if err != nil {
		return fmt.Errorf("createSchema Emulators: %w", err)
	}
	_, err = sqldb.Exec(`
		CREATE TABLE IF NOT EXISTS Games (
			ID              INTEGER PRIMARY KEY AUTOINCREMENT,
			EMUID           INTEGER,
			GameName        TEXT,
			GameFileName    TEXT,
			GameDisplay     TEXT,
			Visible         INTEGER,
			GameYear        TEXT,
			Manufact        TEXT,
			GameType        TEXT,
			DateFileUpdated TEXT,
			WEBGameID       TEXT,
			TAGS            TEXT,
			IPDBNum         TEXT,
			WebLinkURL      TEXT,
			WebLink2URL     TEXT,
			DesignedBy      TEXT,
			Notes           TEXT,
			DateAdded       TEXT,
			DateUpdated     TEXT
		)`)
	if err != nil {
		return fmt.Errorf("createSchema Games: %w", err)
	}
	// Seed a Visual Pinball X emulator row so lookupEMUID succeeds in tests
	// that need a valid emuid (e.g. TestAllGames_FilteredByEMUID).
	_, err = sqldb.Exec(`INSERT INTO Emulators (EmuName) VALUES ('Visual Pinball X')`)
	if err != nil {
		return fmt.Errorf("createSchema seed VPX emulator: %w", err)
	}
	return nil
}

// openTestDB opens an in-memory SQLite DB, creates the schema, and returns
// a *DB ready for testing. Registers t.Cleanup to close.
func openTestDB(t *testing.T) *DB {
	t.Helper()
	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("openTestDB sql.Open: %v", err)
	}
	if err := createSchema(sqldb); err != nil {
		sqldb.Close()
		t.Fatalf("openTestDB createSchema: %v", err)
	}
	// Use unexported newDB to wrap the existing connection so validateColumns
	// sees the schema we just created (plain :memory: DSN creates a fresh DB on
	// each sql.Open call — re-opening with db.Open(":memory:") would be empty).
	d, err := newDB(sqldb)
	if err != nil {
		sqldb.Close()
		t.Fatalf("openTestDB newDB: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// ---------------------------------------------------------------------------
// TestValidateColumns
// ---------------------------------------------------------------------------

func TestValidateColumns(t *testing.T) {
	t.Run("valid schema passes", func(t *testing.T) {
		d := openTestDB(t)
		// validateColumns already ran inside newDB — if we reach here, it passed.
		_ = d
	})

	t.Run("missing column returns error", func(t *testing.T) {
		// Create a DB with a truncated Games table missing required columns.
		// Keep sqldb open so the :memory: DB persists while newDB validates it.
		sqldb, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer sqldb.Close()
		_, err = sqldb.Exec(`CREATE TABLE Games (ID INTEGER PRIMARY KEY, GameName TEXT)`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = sqldb.Exec(`CREATE TABLE Emulators (EMUID INTEGER, EmuName TEXT)`)
		if err != nil {
			t.Fatal(err)
		}
		// newDB runs validateColumns on the existing connection.
		// It should return an error because the Games table is truncated.
		_, openErr := newDB(sqldb)
		if openErr == nil {
			t.Fatal("expected error for missing columns, got nil")
		}
		if !strings.Contains(openErr.Error(), "missing required column") {
			t.Errorf("expected 'missing required column' in error, got: %v", openErr)
		}
	})
}

// ---------------------------------------------------------------------------
// TestUpsertGame_Idempotent
// ---------------------------------------------------------------------------

func TestUpsertGame_Idempotent(t *testing.T) {
	d := openTestDB(t)

	rec := GameRecord{
		GameFileName: "Medieval Madness (Williams, 1997).vpx",
		GameName:     "Medieval Madness",
		GameYear:     "1997",
		Manufact:     "Williams",
		GameType:     "SS",
		IPDBNum:      "3315",
	}

	mtime := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)

	if err := d.UpsertGame(rec, mtime); err != nil {
		t.Fatalf("first UpsertGame: %v", err)
	}

	if err := d.UpsertGame(rec, mtime); err != nil {
		t.Fatalf("second UpsertGame: %v", err)
	}

	// Verify exactly one row exists.
	var count int
	err := d.sql.QueryRow(
		`SELECT COUNT(*) FROM Games WHERE lower(GameFileName) = lower(?)`,
		"Medieval Madness (Williams, 1997).vpx",
	).Scan(&count)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d rows after two UpsertGame calls, want 1", count)
	}
}

// ---------------------------------------------------------------------------
// TestUpsertGame_UpdateSetsDateUpdated
// ---------------------------------------------------------------------------

func TestUpsertGame_UpdateSetsDateUpdated(t *testing.T) {
	d := openTestDB(t)
	rec := GameRecord{
		GameFileName: "Funhouse (Williams, 1990).vpx",
		GameName:     "Funhouse",
		GameYear:     "1990",
		Manufact:     "Williams",
		GameType:     "SS",
	}

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	if err := d.UpsertGame(rec, t1); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := d.UpsertGame(rec, t2); err != nil {
		t.Fatalf("update: %v", err)
	}

	// DateAdded should be set from first insert (non-empty).
	var dateAdded, dateUpdated string
	err := d.sql.QueryRow(
		`SELECT COALESCE(DateAdded,''), COALESCE(DateUpdated,'') FROM Games WHERE lower(GameFileName) = lower(?)`,
		"Funhouse (Williams, 1990).vpx",
	).Scan(&dateAdded, &dateUpdated)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if dateAdded == "" {
		t.Error("DateAdded should be non-empty after insert")
	}
	if dateUpdated == "" {
		t.Error("DateUpdated should be non-empty after update")
	}
}

// ---------------------------------------------------------------------------
// TestBackupBeforeFirstWrite
// ---------------------------------------------------------------------------

func TestBackupBeforeFirstWrite(t *testing.T) {
	// Create a real temp file to use as the "database" to back up.
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "PUPDatabase.db")
	if err := os.WriteFile(dbFile, []byte("fake db content"), 0o644); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(tmpDir, "PUPBackup")

	d := openTestDB(t)

	// First backup: should create a backup file.
	if err := d.BackupBeforeFirstWrite(dbFile, backupDir); err != nil {
		t.Fatalf("first backup: %v", err)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backupDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 backup file, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), "_PUPDatabase.db") {
		t.Errorf("backup filename %q does not end in _PUPDatabase.db", entries[0].Name())
	}

	// Second backup in same session: should be no-op.
	if err := d.BackupBeforeFirstWrite(dbFile, backupDir); err != nil {
		t.Fatalf("second backup: %v", err)
	}
	entries2, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries2) != 1 {
		t.Errorf("expected still 1 backup after second call, got %d", len(entries2))
	}
}

// ---------------------------------------------------------------------------
// TestPruneBackups
// ---------------------------------------------------------------------------

func TestPruneBackups(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 3 backups on the same day; maxPerDay=2 should delete the oldest.
	day := "20260514"
	names := []string{
		day + "100000_PUPDatabase.db",
		day + "110000_PUPDatabase.db",
		day + "120000_PUPDatabase.db",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := pruneBackups(tmpDir, 2, 5); err != nil {
		t.Fatalf("pruneBackups: %v", err)
	}

	remaining, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Errorf("expected 2 files after pruning, got %d", len(remaining))
	}
	// The oldest (100000) should be gone; the newest two kept.
	for _, e := range remaining {
		if e.Name() == day+"100000_PUPDatabase.db" {
			t.Errorf("oldest backup should have been pruned but still exists")
		}
	}
}

// ---------------------------------------------------------------------------
// TestAllGames
// ---------------------------------------------------------------------------

func TestAllGames_FilteredByEMUID(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	vpxEMUID := d.emuid
	if vpxEMUID <= 0 {
		t.Fatalf("test fixture must seed VPX emulator; got emuid %d", vpxEMUID)
	}
	otherEMUID := vpxEMUID + 100

	insert := func(emuid int64, fn, gn string) {
		_, err := d.sql.Exec(`INSERT INTO Games (EMUID, GameName, GameFileName, GameDisplay, Visible,
            GameYear, Manufact, GameType, DateFileUpdated, WEBGameID, TAGS, IPDBNum, WebLinkURL,
            WebLink2URL, DesignedBy, Notes, DateAdded)
            VALUES (?,?,?,?,1,'1992','Williams','SS','','','','','','','','','')`,
			emuid, gn, fn, gn)
		if err != nil {
			t.Fatalf("insert %s: %v", fn, err)
		}
	}
	insert(vpxEMUID, "addams.vpx", "Addams Family")
	insert(vpxEMUID, "twilight.vpx", "Twilight Zone")
	insert(otherEMUID, "other.fp", "Other Game")

	got, err := d.AllGames()
	if err != nil {
		t.Fatalf("AllGames: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("AllGames returned %d rows; want 2 (filtered to VPX EMUID)", len(got))
	}
	seen := map[string]bool{}
	for _, g := range got {
		seen[g.GameFileName] = true
	}
	if !seen["addams.vpx"] || !seen["twilight.vpx"] {
		t.Errorf("missing expected VPX games; got %+v", got)
	}
	if seen["other.fp"] {
		t.Errorf("AllGames returned a row with non-VPX EMUID")
	}
}

func TestAllGames_DegradedNoFilter(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	// Force degraded mode: pretend EMUID lookup failed.
	d.emuid = -1

	_, err := d.sql.Exec(`INSERT INTO Games (EMUID, GameName, GameFileName, GameDisplay, Visible,
        GameYear, Manufact, GameType, DateFileUpdated, WEBGameID, TAGS, IPDBNum, WebLinkURL,
        WebLink2URL, DesignedBy, Notes, DateAdded) VALUES
        (5, 'A', 'a.vpx', 'A', 1, '1992', 'W', 'SS', '', '', '', '', '', '', '', '', '')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := d.AllGames()
	if err != nil {
		t.Fatalf("AllGames degraded: %v", err)
	}
	if len(got) < 1 {
		t.Fatalf("AllGames degraded returned %d rows; want >=1 (no filter applied)", len(got))
	}
}
