// Package db provides a thin client over PUPDatabase (modernc.org/sqlite).
// It is standalone — no import of internal/ui/, internal/executor/, or
// internal/planner/ — to avoid circular imports (D-13). Open accepts any path
// (real or rehearsal); it has no knowledge of rehearsal mode (D-02).
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers "sqlite" driver name (D-11)
)

// GameRecord carries the fields UpsertGame writes to the Games table.
// It is a plain value struct — no dependency on internal/planner/.
// The executor constructs a GameRecord from planner.PlannedAction fields (per D-13).
type GameRecord struct {
	GameFileName string // filepath.Base of the VPX file
	GameName     string
	GameYear     string
	Manufact     string
	GameType     string
	IPDBNum      string
	WebLinkURL   string
	Notes        string
	NumPlayers   string
	Tags         string
	GameTheme    string
	GameRating   string
}

// DB holds an open SQLite connection for the lifetime of one session.
// emuid is cached from the Emulators table at Open time; -1 if not found.
// backed is true after the first backup this session (EXE-06).
type DB struct {
	sql    *sql.DB
	emuid  int64
	backed bool
}

// Open opens the SQLite database at path (real or rehearsal), validates the
// Games table schema, and caches the EMUID for the Visual Pinball X emulator.
// Returns a non-nil error if the Games table is missing required columns.
func Open(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db open %s: %w", path, err)
	}
	return newDB(sqldb)
}

// newDB wraps an already-open *sql.DB, runs validateColumns and lookupEMUID,
// and returns a ready *DB. Used by Open and by tests that pre-populate an
// in-memory database before validation (openTestDB factory).
func newDB(sqldb *sql.DB) (*DB, error) {
	d := &DB{sql: sqldb, emuid: -1}
	if err := d.validateColumns(); err != nil {
		return nil, err
	}
	if err := d.lookupEMUID(); err != nil {
		d.emuid = -1 // non-fatal: REGISTER_GAME actions will fail gracefully
	}
	return d, nil
}

// Close closes the underlying SQLite connection.
func (d *DB) Close() error { return d.sql.Close() }

// validateColumns queries PRAGMA table_info(Games) and verifies that all
// required columns exist. Returns an error for the first missing column.
// Never references GameScan (EXE-07, T-05-01-03).
func (d *DB) validateColumns() error {
	rows, err := d.sql.Query("PRAGMA table_info(Games)")
	if err != nil {
		return fmt.Errorf("PRAGMA table_info: %w", err)
	}
	// Close explicitly before next query (RESEARCH Pitfall 4).
	defer rows.Close()

	found := make(map[string]bool)
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("PRAGMA table_info scan: %w", err)
		}
		found[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("PRAGMA table_info rows: %w", err)
	}
	rows.Close()

	required := []string{
		"EMUID", "GameName", "GameFileName", "GameDisplay",
		"Visible", "GameYear", "Manufact", "GameType",
		"DateFileUpdated", "WEBGameID", "DateAdded", "DateUpdated",
	}
	for _, col := range required {
		if !found[col] {
			return fmt.Errorf("PUPDatabase missing required column: %s", col)
		}
	}
	return nil
}

const emuidSQL = `SELECT EMUID FROM Emulators WHERE EmuName LIKE '%Visual Pinball X%' LIMIT 1`

// lookupEMUID queries the Emulators table for the Visual Pinball X emulator ID
// and caches it in d.emuid. Returns sql.ErrNoRows if no matching emulator found.
func (d *DB) lookupEMUID() error {
	return d.sql.QueryRow(emuidSQL).Scan(&d.emuid)
}

const lookupSQL = `SELECT ID FROM Games WHERE lower(GameFileName) = lower(?) LIMIT 1`

const insertSQL = `
INSERT INTO Games
    (EMUID, GameName, GameFileName, GameDisplay, Visible, GameYear,
     Manufact, GameType, DateFileUpdated, WEBGameID,
     TAGS, IPDBNum, WebLinkURL, WebLink2URL, DesignedBy, Notes, DateAdded)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

const updateSQL = `
UPDATE Games SET
    EMUID=?, GameName=?, GameDisplay=?, Visible=?, GameYear=?,
    Manufact=?, GameType=?, DateFileUpdated=?, WEBGameID=?,
    TAGS=?, IPDBNum=?, WebLinkURL=?, WebLink2URL=?,
    DesignedBy=?, Notes=?, DateUpdated=?
WHERE ID=?`

// UpsertGame inserts or updates a Games row for rec.GameFileName.
// Uses SELECT + conditional INSERT/UPDATE (never INSERT OR REPLACE) to
// preserve PinUP Popper-managed columns (EXE-05, T-05-01-02).
// All SQL parameters use bind variables — no string interpolation (T-05-01-01).
func (d *DB) UpsertGame(rec GameRecord, mtime time.Time) error {
	gameName := rec.GameName
	if gameName == "" {
		gameName = filepath.Base(rec.GameFileName)
	}
	gameType := rec.GameType
	if gameType == "" {
		gameType = "EM"
	}
	dateFileMod := mtime.Format("2006-01-02 15:04:05")

	var id int64
	err := d.sql.QueryRow(lookupSQL, rec.GameFileName).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// New row — use DateAdded.
		dateAdded := time.Now().Format("2006-01-02 15:04:05")
		_, execErr := d.sql.Exec(insertSQL,
			d.emuid,
			gameName,
			rec.GameFileName,
			gameName,     // GameDisplay = GameName
			1,            // Visible
			rec.GameYear,
			rec.Manufact,
			gameType,
			dateFileMod,
			rec.IPDBNum,  // WEBGameID
			rec.Tags,     // TAGS
			rec.IPDBNum,  // IPDBNum
			rec.WebLinkURL,
			"",           // WebLink2URL
			"",           // DesignedBy
			rec.Notes,
			dateAdded,
		)
		if execErr != nil {
			return fmt.Errorf("db insert game %s: %w", rec.GameFileName, execErr)
		}
		return nil
	case err != nil:
		return fmt.Errorf("db lookup game %s: %w", rec.GameFileName, err)
	default:
		// Existing row — use DateUpdated.
		dateUpdated := time.Now().Format("2006-01-02 15:04:05")
		_, execErr := d.sql.Exec(updateSQL,
			d.emuid,
			gameName,
			gameName,     // GameDisplay = GameName
			1,            // Visible
			rec.GameYear,
			rec.Manufact,
			gameType,
			dateFileMod,
			rec.IPDBNum,  // WEBGameID
			rec.Tags,     // TAGS
			rec.IPDBNum,  // IPDBNum
			rec.WebLinkURL,
			"",           // WebLink2URL
			"",           // DesignedBy
			rec.Notes,
			dateUpdated,
			id,
		)
		if execErr != nil {
			return fmt.Errorf("db update game %s: %w", rec.GameFileName, execErr)
		}
		return nil
	}
}

// GameRow is a read-only projection of one Games table row, sufficient for
// the monitor-mode three-section gap report (MOD-07). Defined here (not in
// app/) because the columns are owned by the db package's schema knowledge.
type GameRow struct {
	GameID       int64
	GameFileName string
	GameName     string
	EMUID        int64
}

const allGamesSQL = `SELECT ID, GameFileName, GameName, EMUID FROM Games WHERE EMUID = ?`
const allGamesNoFilterSQL = `SELECT ID, GameFileName, GameName, EMUID FROM Games`

// AllGames returns every row from the Games table belonging to the cached
// Visual Pinball X emulator (d.emuid). If lookupEMUID failed at Open() time
// (d.emuid == -1), AllGames returns all rows with no EMUID filter — a
// graceful-degradation fallback so single-emulator machines without a named
// VPX entry still get a useful monitor report (RESEARCH Open Question #3).
//
// MOD-07: caller compares the slice against catalog.Entries() to produce the
// Not-Installed / Not-in-Catalog / Name-Mismatch sets.
func (d *DB) AllGames() ([]GameRow, error) {
	var rows *sql.Rows
	var err error
	if d.emuid == -1 {
		rows, err = d.sql.Query(allGamesNoFilterSQL)
	} else {
		rows, err = d.sql.Query(allGamesSQL, d.emuid)
	}
	if err != nil {
		return nil, fmt.Errorf("db AllGames query: %w", err)
	}
	defer rows.Close()

	var out []GameRow
	for rows.Next() {
		var g GameRow
		if err := rows.Scan(&g.GameID, &g.GameFileName, &g.GameName, &g.EMUID); err != nil {
			return nil, fmt.Errorf("db AllGames scan: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db AllGames rows: %w", err)
	}
	return out, nil
}

