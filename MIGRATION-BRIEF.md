# vPinManager — Migration Brief

**Target audience:** Go developer rebuilding this tool from scratch.  
**Purpose:** Extract intent, domain knowledge, and design decisions precisely enough
to reimplement without reading the original code.  
**Scope:** All three modes are covered. Only the `process` mode is fully implemented
in the Python version; `monitor` and `download` are stubs but their intent is documented.

---

## 1. What This Application Does

vPinManager is a command-line tool for people who operate a virtual pinball cabinet.
A virtual pinball cabinet runs Visual Pinball X (VPX) and a front-end called PinUP
Popper that provides a menu UI. When a player downloads a new pinball table or
companion content (backglass art, ROM chip set, DMD colorization, etc.), those files
arrive in a flat `downloads/` directory, often packed inside distribution archives, with
inconsistent naming conventions that differ from what VPX and PinUP Popper expect.
vPinManager's job is to scan that directory, identify every asset by type, rename and
move each one to the correct location in the VPX install layout, and register newly
installed tables in PinUP Popper's SQLite database — all without touching anything
until the user reviews and confirms a dry-run plan.

The central UX decision is **plan-then-confirm**. Every run produces a complete list
of proposed actions before asking the user to execute. The default mode is dry-run;
the user must explicitly approve or pass `--auto` to trigger execution. During the
planning phase, when a file cannot be matched to the catalog automatically, the user
is shown up to five fuzzy-matched candidates with confidence scores and can select one,
force a search by name or ID, send the file to a holding area for later review, or
ignore it entirely.

The catalog is a publicly maintained Google Sheet listing every known virtual pinball
recreation, organized by manufacturer. It provides stable identifiers (MasterID,
IPDB number), canonical game names, and metadata (year, game type, designer) that the
tool uses to rename files correctly and populate the database. The catalog is cached
locally as an Excel export and refreshed on demand.

Two additional modes are planned but not yet implemented: `monitor` (compare the local
installed collection against the catalog to identify gaps) and `download` (open browser
URLs for the next incomplete section of the collection).

---

## 2. Core Domain Concepts

**Virtual Pinball Table.** A software recreation of a physical pinball machine, played
on a cabinet with a vertical playfield monitor and a horizontal backbox monitor. The
primary file is a `.vpx` script; it is typically accompanied by a backglass image
(`.directb2s`), a point-of-view configuration (`.ini`), a ROM chip set (`.zip`), and
optionally DMD colorization, alternate sound packs, and UltraDMD/FlexDMD video packs.
These companion files must share the exact same filename stem as the `.vpx` file for
VPX and PinUP Popper to associate them automatically.

**Canonical Filename.** The standard stem format is `{name} ({manufacturer}, {year})`,
e.g. `PIN-BOT (Williams, 1986)`. This is derived from catalog fields, not from the
download filename. Renaming to this form is mandatory before installation. The `name`
component follows the trailing-article convention: "The Addams Family" becomes
"Addams Family, The" to match how the catalog stores it.

**Catalog.** A curated Google Sheet (Sheet ID `12-Pwub-p4krv17cwOFT4kr4qR_a34B_YbBvHjEcpSI0`)
with one tab per manufacturer. Each row describes one known table recreation. The
canonical source is the xlsx export, downloaded on demand and cached locally. The
catalog is the only way to determine what canonical name and metadata to assign to
a downloaded file. It must never silently fall back to empty — a missing cache is an
error the user must resolve.

**SheetEntry.** One row from the catalog. Fields: `game_name` (raw sheet value),
`name` (extracted title only), `manufacturer`, `year`, `game_type`, `master_id`
(stable unique key), `ipdb_num` (IPDB database number), `designed_by`, `decade`,
`tier`, `notes`, `vpw_link` (set if a VPW community version exists), `vps_link`,
`ipdb_url`. The `canonical_filename` property constructs `{name} ({manufacturer}, {year})`.

**MatchResult.** The output of fuzzy matching one file stem against the catalog.
Contains the matched `SheetEntry`, a `confidence` score (0–100), and a `match_field`
indicating which field drove the match (`"game_name"`, `"master_id"`, `"ipdb_num"`).

**ProcessPlan.** The complete description of all work to be done for one `process` run.
Contains a list of top-level `PlannedAction`s; some actions (archive extractions) have
children. The plan is a tree, not a flat list. The plan is fully constructed before
any filesystem writes occur.

**PlannedAction.** One node in the plan tree. Has an action type, a source path, an
optional destination path, an optional `MatchResult`, a human-readable reason, and a
list of child actions. Archives hold their children (vault action + per-member actions)
inside this node. Also carries `virtual_path` (display path relative to downloads dir,
e.g. `archive.zip/game.vpx`), `superseded_by` (set by deduplication on losers), and
`register_game` (back-pointer from a MOVE_VPX to its paired REGISTER_GAME — used by
deduplication to kill orphaned DB writes).

**Downloads Directory.** The input directory of files to process. May contain loose
files, loose bundle directories (e.g. a PuP pack), and distribution archives. The tool
supports arbitrary nesting of archives (nested archives inside archives are recursively
extracted at execution time). After processing, the directory should be empty or
contain only un-processable items.

**Install Layout (Baller Installer standard).** The fixed directory structure expected
by VPX and PinUP Popper:
```
C:\vPinball\visualpinball\
  Tables\           .vpx, .directb2s, .ini, FlexDMD dirs
  VPinMAME\roms\    ROM .zip files (unextracted)
  VPinMAME\nvram\   .nv NVRAM save files
  altcolor\         DMD colorization directories
  altsound\         alternate sound pack directories
  UltraDMD\         UltraDMD video pack directories
  Music\            music track directories
C:\vPinball\PinUPSystem\
  POPMedia\         PuP pack directories
  PUPDatabase.db    SQLite database
  PUPBackup\        automatic backups
```

**PinUP Popper.** The front-end software that presents the table menu. It reads
`PUPDatabase.db` to discover installed tables. Every installed `.vpx` file must have
a corresponding row in the `Games` table with the correct `EMUID`, filename, and
metadata. PinUP Popper also owns a `GameScan` table — this tool must never write to it.

**Review Holding Area.** Files that cannot be matched or that the user explicitly
defers go to `review/` at the project root. A `REVIEW.md` log is appended (never
replaced) with a timestamped table showing rejected filename, chosen-instead filename
(for duplicates), and the reason.

**Rehearsal Mode.** A sandboxed run where downloads are copied to `rehearsal/downloads/`,
all install destinations are remapped to `rehearsal/` subdirectories, and PUPDatabase
is copied to `rehearsal/PUPDatabase.db`. Safe to run at any time; does not touch the
real install layout.

---

## 3. Asset Types — CRITICAL

### 3.1 VPX Tables

- **Represents:** The primary game file — a Visual Pinball X script that runs the
  complete simulation of one pinball machine.
- **Identification:** Extension `.vpx`. No content inspection required; the extension
  is unambiguous. Case-insensitive extension check.
- **Action:** Rename to canonical stem if needed, move to `Tables\`, insert or update
  a row in the `Games` table of PUPDatabase.
- **Appears:** Loose in downloads, inside distribution archives. Never inside bundle
  directories.
- **Dependencies:** Should be accompanied by a matching `.directb2s`, `.ini`, ROM `.zip`,
  and optionally DMD/sound/music packs — all keyed to the same canonical filename stem.
- **Name matching required:** Yes. Fuzzy-matched against catalog. Auto-assigned at ≥92%
  confidence; user prompted at 72–91%; sent to review below 72% (non-interactive) or
  per user choice (interactive).

### 3.2 Backglasses

- **Represents:** The backglass artwork file displayed on the backbox monitor. Loaded
  by Visual Pinball X's B2S server by matching the stem to the companion `.vpx`.
- **Identification:** Extension `.directb2s`. Unambiguous.
- **Action:** Rename to canonical stem, move to `Tables\`.
- **Appears:** Loose, inside distribution archives.
- **Dependencies:** Must share exact stem with its companion `.vpx`.
- **Name matching required:** Yes, same fuzzy-match pipeline as tables.

### 3.3 POV Files (Point-of-View configurations)

- **Represents:** A per-table `.ini` file that stores camera angles and perspective
  settings for VPX. Loaded by VPX by matching the stem to the companion `.vpx`.
- **Identification:** Two-stage: (1) extension `.ini`, (2) content inspection — the file
  must contain the text `[TableOverride]` or `[Player]` anywhere in its body. Files
  with `.ini` extension but without these headers are not POV files and go to review.
  **Exception:** Archive member paths do not exist on disk at plan time; content
  inspection is skipped for archive members, and the extension alone is trusted.
- **Action:** Rename to canonical stem, move to `Tables\`.
- **Appears:** Loose, inside distribution archives.
- **Dependencies:** Must share exact stem with its companion `.vpx`.
- **Name matching required:** Yes.

### 3.4 ROM Archives

- **Represents:** A zip archive containing the chip binary ROM files for VPinMAME (the
  pinball ROM emulator). VPinMAME reads the archive directly without extracting it.
- **Identification:** Extension `.zip` with content inspection. A `.zip` is a ROM if
  and only if: (a) its member extensions intersect the ROM chip extension set
  `{.bin, .rom, .cpu, .snd, .u06, .u07, .u08, .u09}`, **and** (b) it contains **none**
  of the VPX asset extensions `{.vpx, .directb2s, .ini, .mp3, .png, .jpg}`.
  Without content inspection, a `.zip` defaults to `ARCHIVE` (distribution archive).
  ROM zips typically have short ROM code names: `mm_109c.zip`, `afm_113.zip`.
- **Action:** Copy as-is to `VPinMAME\roms\`, preserving the original filename.
  **Do not extract.**
- **Appears:** Loose, inside distribution archives.
- **Name matching required:** No. ROM filenames are opaque chip codes with no
  relationship to canonical table names.

### 3.5 NVRAM Saves

- **Represents:** Non-volatile RAM save files for VPinMAME. Stores high scores and
  machine settings. Filename is the ROM code (e.g. `mm_109c.nv`).
- **Identification:** Extension `.nv`. Unambiguous.
- **Action:** Copy to `VPinMAME\nvram\`, preserving original filename.
- **Appears:** Loose, inside distribution archives.
- **Name matching required:** No.

### 3.6 Music Directories

- **Represents:** A directory of audio tracks for table-specific background music.
  VPX loads music from `Music\{dirname}\`.
- **Identification:** Directory-level detection. A directory is a Music bundle if:
  its member extensions intersect `{.mp3, .ogg, .wav, .flac}`, **and** it does not
  contain `screens.pup` (which would make it a PuP pack sub-directory).
- **Action:** Move the entire directory to `Music\{dirname}`, preserving the directory
  name exactly.
- **Appears:** Loose on disk, inside distribution archives (as a top-level directory).
- **Name matching required:** No. Directory name is the music pack name as intended by
  its creator.

### 3.7 Altcolor (DMD Colorization) Directories

- **Represents:** Per-ROM colorization data that replaces the monochrome DMD display
  with full color. Multiple formats exist, all stored in a directory named for the ROM
  code (e.g. `mm_109c\`).
- **Identification:** Directory-level detection. Three mutually exclusive sub-formats:
  - **Serum format:** Any `.cRZ` file present in the directory.
  - **Pin2DMD (VNI/PAL) format:** Both `.vni` **and** `.pal` files present (both required).
  - **PAC format:** Any `.pac` file present.
  If none of these signals are present, the directory is not an altcolor bundle.
- **Action:** Move the entire directory to `altcolor\{dirname}`.
- **Appears:** Loose on disk, inside distribution archives.
- **Name matching required:** No.
- **Note:** Single loose `.cRZ` or `.pac` files (outside a directory) are classified as
  `ALTCOLOR` by extension but treated as unknown types in the planner (sent to review),
  because the practical delivery unit is always a directory.

### 3.8 Altsound (Alternate Audio) Directories

- **Represents:** Replacement audio packs for VPinMAME. Each pack replaces some or all
  ROM-triggered sounds with higher-quality audio.
- **Identification:** Directory-level detection. Definitive signal is the presence of
  **one of these specific CSV files** in the directory root: `altsound.csv` (legacy
  format) or `g-sound.csv` (G-Sound/PinSound format). Audio files alone are **not**
  sufficient because audio files also appear in PuP pack and Music directories.
- **Action:** Move the entire directory to `altsound\{dirname}`.
- **Appears:** Loose on disk, inside distribution archives.
- **Name matching required:** No.

### 3.9 UltraDMD Directories

- **Represents:** A video asset pack for the UltraDMD DMD renderer. Loaded by VPX by
  matching the directory name to the table stem (including the `.UltraDMD` suffix).
- **Identification:** Directory-level detection. Definitive signal: directory name ends
  with `.UltraDMD` (case-sensitive suffix check).
- **Action:** Move the entire directory to `UltraDMD\{dirname}`, preserving the full
  name including the `.UltraDMD` suffix.
- **Appears:** Loose on disk, inside distribution archives.
- **Name matching required:** No.
- **Example:** `Medieval Madness (Williams, 1997).UltraDMD\`

### 3.10 FlexDMD Directories

- **Represents:** A video asset pack for the FlexDMD DMD renderer. Unlike UltraDMD,
  FlexDMD directories live alongside the `.vpx` in `Tables\`.
- **Identification:** Directory-level detection. Definitive signal: directory name ends
  with `.FlexDMD` (case-sensitive suffix check).
- **Action:** Move the entire directory to `Tables\{dirname}` (not a dedicated root —
  alongside the `.vpx`).
- **Appears:** Loose on disk, inside distribution archives.
- **Name matching required:** No.
- **Example:** `Medieval Madness (Williams, 1997).FlexDMD\`

### 3.11 PuP Packs

- **Represents:** A PinUP Popper video pack that plays video on auxiliary screens
  during gameplay. Managed entirely by PinUP Popper; matched by directory name.
- **Identification:** Directory-level detection. Definitive signal: file named
  `screens.pup` present in the directory root. The directory may also contain
  subdirectories like `Videos\`, `Images\`, `Scripts\`, `Audio\` — but `screens.pup`
  is the only reliable anchor. Directory name suffix checks are not used.
- **Action:** Move the entire directory to `PinUPSystem\POPMedia\{dirname}`.
- **Appears:** Loose on disk, inside distribution archives.
- **Name matching required:** No. PinUP Popper resolves packs by exact directory name.

### 3.12 Distribution Archives

- **Represents:** A compressed container wrapping any combination of the above asset
  types. Not itself a content type — it is a delivery mechanism.
- **Identification:** Extensions `.zip`, `.rar`, `.7z`. A `.zip` is only treated as a
  distribution archive (vs ROM archive) when content inspection reveals it does not
  meet the ROM criteria.
- **Action:** Extract to a subdirectory named after the archive stem (`archive.zip` →
  `archive/`), plan each member individually, vault the original to `archive_vault/`.
- **Appears:** Loose at any level of nesting.
- **Special case:** A `.zip` found inside a distribution archive that was not classified
  as a ROM gets stubbed as `EXTRACT_ARCHIVE` during planning and is recursively
  extracted at execution time.

---

## 4. Archive & File Traversal

### Supported formats

| Format | Extension | Peek library | Extract library |
|--------|-----------|--------------|-----------------|
| ZIP    | `.zip`    | `archive/zip` stdlib | `archive/zip` stdlib |
| 7-Zip  | `.7z`     | `py7zr` (Python) | `py7zr` |
| RAR    | `.rar`    | `rarfile` (Python) | `rarfile` |

In Go: use `archive/zip` from stdlib; `github.com/bodgit/sevenzip` or shelling to 7z
for 7-Zip; `github.com/nwaples/rardecode` or shelling to unrar for RAR.

### Traversal algorithm

The scan runs in two passes over `downloads_dir`:

**Pass 1 — loose directory bundles:** Iterate direct children of `downloads_dir` that
are directories. For each, collect all files recursively and attempt bundle
classification (Section 3). Recognised bundles are added to the plan and their
directory is marked as claimed. This pass must run before the file walk so that files
inside bundle directories are not processed again individually.

**Pass 2 — file walk:** `rglob("*")` over `downloads_dir`, sorted. For each file:
- Skip if inside a claimed bundle directory.
- Skip if inside a directory that was the extract target of an already-planned archive
  (prevents re-processing extracted content during planning when archives have already
  been partially extracted from a prior aborted run).
- If the extension is `.zip`, `.rar`, or `.7z`: peek the member list, classify as ROM
  or distribution archive, plan accordingly.
- Otherwise: plan the loose file.

### Archive peeking (planning phase)

During planning, archives are never extracted. Instead, the member list is read:
- **ZIP:** `ZipFile.namelist()` — returns flat list of paths including directory entries.
- **7z:** `SevenZipFile.getnames()`.
- **RAR:** `RarFile.namelist()`.

Member paths use forward slashes. Directory entries end in `/` or `\` and are skipped
during grouping.

### Member classification inside archives

Archive members are grouped before individual classification:

1. Split member list into top-level directory groups (first path component) and loose files.
2. For each top-level directory, collect relative member names and attempt bundle
   classification using the same `_classify_directory` logic as Pass 1.
3. If a top-level directory is recognised as a bundle, all its members are treated as
   one unit — individual files inside are **never** classified separately.
4. If a top-level directory is **not** recognised, it is "dissolved" — each of its files
   is classified individually by extension as if it were a loose file.
5. Loose files (no containing directory) are classified by extension.

**Why directory-first matters:** A `.ini` inside a PuP pack must not be mistaken for
a POV file. An `.mp3` inside an AltSound directory must not be mistaken for Music.
The bundle check is the guard.

### Nested archives

A `.zip` (or other archive) found as a member inside a distribution archive is stubbed
as `EXTRACT_ARCHIVE` during planning (because the nested archive doesn't exist on disk
yet — it will only appear after the outer archive is extracted). During execution, after
all planned members are processed, the execution engine scans the extract root for any
remaining archive files and recursively extracts them in place. Nested archives do not
get a vault action — they are deleted after extraction.

### Edge cases explicitly handled

- **ROM vs distribution ZIP:** Content inspection distinguishes them (Section 3.4). This
  must happen before the archive is planned as a distribution archive.
- **Archive extract root collision with bundle directories:** A claimed bundle directory
  from Pass 1 prevents files inside it from being picked up by Pass 2.
- **Duplicate destinations (deduplication):** Multiple sources mapping to the same
  canonical destination are resolved post-planning. The winner is chosen by (1) higher
  match confidence, (2) newer file mtime. Losers are rerouted to review with a
  human-readable explanation. Archive members use the parent archive's mtime as the
  proxy because extracted paths don't exist until execution. The deduplication also
  follows a back-pointer from each `MOVE_VPX` action to its paired `REGISTER_GAME`
  action and sets the loser's `REGISTER_GAME` to `SKIP`, preventing phantom DB writes.
- **Mixed-content archives:** Archives containing multiple asset types (e.g. `.vpx` +
  `.directb2s` + ROM `.zip`) are fully supported. Each member is classified and planned
  independently. An inner `.zip` that looks like a ROM is classified as ROM (verbatim
  install); one that doesn't is treated as a nested archive (extract recursively).

---

## 5. Application States & Lifecycle

```
             ┌─────────────────────────────────────────────┐
             │              vpin process                   │
             └──────────────────────┬──────────────────────┘
                                    │
                              ┌─────▼──────┐
                              │    IDLE    │
                              └─────┬──────┘
                                    │  load catalog
                              ┌─────▼──────┐
                              │  LOADING   │  (download if stale/missing)
                              └─────┬──────┘
                                    │  scan + peek archives
                              ┌─────▼──────┐
                              │  SCANNING  │  ←──────────────────┐
                              └─────┬──────┘                     │
                                    │  file needs user input      │
                              ┌─────▼──────┐                     │
                              │  MATCHING  │  (interactive only)  │
                              └─────┬──────┘                     │
                                    │  user decides ─────────────┘
                                    │  all files planned
                              ┌─────▼──────┐
                   dry-run ───│    IDLE    │ (report printed; exit)
                              └─────┬──────┘
                                    │  user confirms execute
                              ┌─────▼──────┐
                              │ EXECUTING  │  (move files, write DB)
                              └─────┬──────┘
                                    │
                              ┌─────▼──────┐
                              │    DONE    │  (terminal state; display summary)
                              └────────────┘
```

**Modes:**
- `--dry-run` (default): exits after report, never reaches EXECUTING.
- `--auto`: skips MATCHING entirely; files below threshold go to review automatically;
  execution proceeds without confirmation prompt.
- `--rehearsal`: EXECUTING writes to a sandboxed copy; real install layout untouched.

**Error behavior:**
- Catalog not found: fatal error with actionable message; user must download.
- Per-action failure during EXECUTING: logged, counted in `failed`, plan continues.
  Other actions are not aborted.
- Archive extraction failure: logged, archive's children are skipped.
- DB write failure: logged, counted in `failed`, plan continues.
- Ctrl+C on any picker: clean exit with "Cancelled." message and `SystemExit(0)`.
- Ctrl+C on text search input: cancel back to picker (not an exit).

---

## 6. Inputs & Outputs

### Inputs

| Source | Key | Default | Notes |
|--------|-----|---------|-------|
| CLI flag | `--db` | `C:\vPinball\PinUPSystem\PUPDatabase.db` | Path to PUPDatabase SQLite file |
| CLI flag | `--dir` | `{project_root}\downloads` | Directory of files to process |
| CLI flag | `--auto` | false | Skip interactive matching prompts |
| CLI flag | `--dry-run` | false | Show plan without executing |
| CLI flag | `--rehearsal` | false | Execute against sandboxed copy |
| File | `vpinmanager/cache/catalog.xlsx` | — | Local catalog cache; fetched from Google Sheets; git-ignored |
| Network | Google Sheets export URL | — | `https://docs.google.com/spreadsheets/d/{SHEET_ID}/export?format=xlsx` |

### Outputs

| Output | Condition | Notes |
|--------|-----------|-------|
| Files moved to install dirs | execution only | See install layout in Section 2 |
| Files moved to `review/` | always (planning decides) | Unmatched or user-deferred files |
| Files moved to `ignored/` | always | User-ignored files |
| Archives moved to `archive_vault/` | execution only | Original archives kept after extraction |
| `review/REVIEW.md` | execution + review items exist | Appended, never replaced; markdown table per run |
| `PUPDatabase.db` mutations | execution + matched tables | Insert/update `Games` rows only; `GameScan` never touched |
| `PUPBackup/{timestamp}_PUPDatabase.db` | first write per session | Timestamp-first for natural sort (`YYYYMMDDHHMMSS`) |
| Terminal output | always | Plan report, execution log, summary counts |
| Exit code | always | 0 = success or user quit; non-zero on unhandled exception |

### Backup rotation policy

- Keep at most 5 backups per calendar day.
- Keep at most 5 days of backups.
- Applied after each backup is created (so the first write of a new day prunes old days).

---

## 7. What the Current Implementation Got Right

**Plan-then-execute separation.** Building a complete `ProcessPlan` before touching
any files is the correct architecture. It enables dry-run mode, makes testing trivial
(tests assert against the plan, not filesystem state), and makes the interactive
confirmation step natural. Carry this forward.

**Dry-run as the default.** Execution requires explicit opt-in. This is the right
default for a tool that moves files and writes a database.

**Same-era filtering for fuzzy matching.** When a filename already contains
`(Manufacturer Year)`, restricting candidates to the same manufacturer and year before
scoring produces near-perfect results on correctly-named files. Without this, the shared
manufacturer/year suffix inflates scores across different tables from the same era.
The fallback to full-catalog scoring when no same-era candidates are found is also
correct.

**Normalization pipeline before scoring.** CamelCase splitting, Roman numeral
conversion, number word conversion, and possessive stripping all run before fuzzy
scoring, on both the query and the catalog entry. This makes `"GrandPrix"` match
`"Grand Prix"` and `"VIII"` match `"8"`. The pipeline is config-toggleable for
debugging — good design.

**Directory-first classification inside archives.** Bundle directories (PuP packs,
altcolor packs, etc.) must be classified as a unit before their contents are
individually classified. This prevents false positives from `.ini` files inside PuP
packs and `.mp3` files inside AltSound packs.

**Deduplication with back-pointer.** When the `MOVE_VPX` action carries a direct
object reference to its paired `REGISTER_GAME` action, deduplication can follow the
pointer rather than matching by source path. This is correct and robust: path-based
matching breaks when multiple archive members rename to the same canonical path.

**Canonical filenames derived from catalog fields.** The name used for renaming comes
from authoritative catalog data, not from any heuristic applied to the download
filename. This ensures consistency regardless of how files are named by their
distributors.

**Append-only review log.** `REVIEW.md` is appended with a timestamped section per
run, never replaced. A user can review multiple sessions' output in one file.

**Rehearsal mode.** A sandboxed dry-execution against copied files and a copied
database is the correct way to let users verify behavior before committing to a real
run. The path-remapping approach (rewrite `dest` fields in the plan after building it)
is elegant — no rehearsal logic bleeds into the plan builder.

---

## 8. What the Current Implementation Got Wrong

**Two processes own the same directory structure.** The install layout paths are
hardcoded for a single known machine (`C:\vPinball\...`). Any user with a different
install root must edit `config.py`. The Go rebuild should discover paths from the
PUPDatabase or from a config file, not hardcode them.

**Catalog freshness threshold is 1 minute.** The current code treats a catalog older
than 60 seconds as "stale" and prompts to refresh. This is aggressive; a daily or
weekly threshold is more appropriate for a catalog that changes infrequently.

**Progress state is duplicated.** `_PROGRESS_CORE = ("VPX", "B2S", "INI", "ROM")` is
defined independently in both `tui.py` and `process.py`. This is an artifact of
incremental development. The Go rebuild should have a single source of truth for
display metadata.

**Archive extraction is not streamed for RAR/7z.** ZIP extraction advances a progress
counter per member. RAR and 7z extraction is atomic — the extraction completes as one
operation and then the counter is set to complete. This is a library limitation but the
Go implementation could do better.

**The DB schema is assumed, not discovered.** Column names like `EMUID`, `GameFileName`,
`WEBGameID` are hardcoded strings. If PinUP Popper changes its schema, the tool breaks
silently. The Go rebuild should validate that expected columns exist before attempting
writes.

**`_plan_file` is called for archive members before they exist on disk.** The POV
content-inspection check (`is_pov_ini`) is skipped for archive member paths because
the file isn't extracted yet. This means archive members with `.ini` extensions are
trusted on extension alone. The Go rebuild should either defer POV classification to
execution time, or accept this limitation explicitly (it is acceptable in practice).

**Single-process global state for the DB client.** `_db_path`, `_backed_up`, etc. are
module-level variables mutated by `init()`. This is not safe in a concurrent context.
The Go rebuild should make DB state explicit (a struct with methods).

---

## 9. File Format Handling — PRESERVE WITH MAXIMUM FIDELITY

### 9.1 Shared interface

Every format handler must support two operations:
1. **Peek** — return the flat member list without extracting.
2. **Extract** — extract all members to a destination directory.

Peek is called during planning; extract is called during execution. They must be
independent — peek must not partially extract.

---

### 9.2 ZIP (`.zip`)

**Detection:** Extension `.zip`, case-insensitive.

**Sub-classification (ROM vs distribution archive):**
Open the zip and collect the set of all member extensions (lowercased):
- If `memberExts ∩ {".bin",".rom",".cpu",".snd",".u06",".u07",".u08",".u09"}` is non-empty,
  **and** `memberExts ∩ {".vpx",".directb2s",".ini",".mp3",".png",".jpg"}` is empty
  → ROM archive (install verbatim, do not treat as distribution archive).
- Otherwise → distribution archive (peek members, plan each member).

Sub-classification requires peek. Without peek (e.g. path-only input), default to
distribution archive.

**Peek:** Return `ZipReader.File[*].Name` for all entries. Include directory entries
(names ending in `/`) — the grouper filters them. Member paths use forward slashes.

**Extract:** Extract each member to `dest/`, creating parent directories as needed.
Member count is known in advance; extraction can be reported per-member.

**Edge cases:**
- Zip entries may have path separators that differ (`\` vs `/`); normalize to `/`
  when returning member names for classification.
- Zip slip: validate that extracted paths remain under `dest/`.

---

### 9.3 7-Zip (`.7z`)

**Detection:** Extension `.7z`, case-insensitive.

**Peek:** Return the full list of entry names. The `py7zr` library returns paths with
forward slashes.

**Extract:** Extract all to `dest/`. 7z extraction is atomic in most Go libraries —
there is no per-entry callback, so progress reporting shows completion only at the end.

**Note:** No ROM sub-classification for `.7z` — ROM archives are always `.zip` by
convention in the VPX community.

---

### 9.4 RAR (`.rar`)

**Detection:** Extension `.rar`, case-insensitive.

**Peek:** Return the member name list.

**Extract:** Extract all to `dest/`. Atomic — no per-entry progress.

**Note:** No ROM sub-classification for `.rar`.

**Known limitation:** RAR extraction requires an external `unrar` binary or a Go
library with RAR5 support. The Python implementation uses `rarfile`, which shells out
to `unrar`. The Go implementation should handle the case where the binary is absent
and report it clearly rather than silently failing.

---

### 9.5 Excel Catalog (`.xlsx`)

**Source:** Google Sheets export of Sheet ID `12-Pwub-p4krv17cwOFT4kr4qR_a34B_YbBvHjEcpSI0`
as xlsx format. URL: `https://docs.google.com/spreadsheets/d/{SHEET_ID}/export?format=xlsx`

**Detection:** `LOCAL_CATALOG_XLSX` path configured in config. If the file does not
exist, raise an error — never return an empty catalog silently.

**Parsing:**
1. Open the workbook. Iterate all sheet tabs.
2. For each tab, read row 1 as headers.
3. Skip any tab where `"GameName"` is not in the header row — these are non-data
   tabs (Index, About, etc.).
4. For each data row (row 2 onward):
   - Skip rows where all cells are null/empty.
   - Read `GameName`, `Manufact`, `GameYear`, `GameType`, `MasterID`, `IPDBNum`,
     `DesignedBy`, `Decade`, `Tier`, `Notes`, `VPW Version Link`, `VPS Link`,
     `WebLinkURL`.
   - Parse `GameYear` as `int` (via float conversion to handle Excel numeric
     representation — `int(float(raw))`).
   - Parse `IPDBNum` as string (some IPDB numbers are integers in the sheet, some
     are blank). Convert via `str(int(float(raw)))` when parseable, else use raw
     string.
   - Extract `name` from `game_name` using `extract_signal` (see Section 9.6).
     If no signal is found, use the full `game_name` as `name`.

**Caching:** Write the downloaded bytes to `vpinmanager/cache/catalog.xlsx` (or
equivalent config path). An in-process LRU cache holds the parsed list for the
duration of one run — the catalog is parsed once, not once per query.

**Refresh trigger:** If the local file is absent, prompt to download (default: Yes).
If the local file is older than the configured threshold, prompt to download (default:
No). In `--auto` mode, never prompt; use existing catalog if present.

---

### 9.6 Filename Normalization (naming.go)

All normalization is applied before any fuzzy scoring, on both the query and the
catalog entry name. **Must run before lowercasing** — Roman numeral detection is
case-sensitive.

**Possessive stripping:** Remove `'s` or `'s` (straight or curly apostrophe) followed
by a word boundary. Pattern: `['']s\b` (case-insensitive).
Example: `Harley's → Harley`, `Harley's → Harley`.

**CamelCase splitting:** Insert a space at CamelCase boundaries.
Rules: space before any uppercase letter preceded by a lowercase letter;
space before any uppercase letter that is followed by a lowercase letter and preceded
by an uppercase letter.
Regex boundaries: `(?<=[a-z])(?=[A-Z])` and `(?<=[A-Z])(?=[A-Z][a-z])`.
Example: `GrandPrix → Grand Prix`, `VPXGame → VPX Game`.

**Roman numeral conversion (II–XX only, I excluded):** Match whole-word Roman numerals
and replace with the decimal equivalent. The letter `I` is excluded because it is too
ambiguous as a standalone word. Process longest numerals first to avoid partial matches.
Range and values:
```
II=2, III=3, IV=4, V=5, VI=6, VII=7, VIII=8, IX=9, X=10,
XI=11, XII=12, XIII=13, XIV=14, XV=15, XVI=16, XVII=17, XVIII=18, XIX=19, XX=20
```
Example: `Space Shuttle III → Space Shuttle 3`.

**Number word conversion (0–12, cardinals + ordinals):** Replace whole-word number
words with digits. Case-insensitive. Ordinals: first=1, second=2, third=3, fourth=4,
fifth=5, sixth=6, seventh=7, eighth=8, ninth=9, tenth=10.
Cardinals: zero=0, one=1, two=2, three=3, four=4, five=5, six=6, seven=7, eight=8,
nine=9, ten=10, eleven=11, twelve=12.
Process longest words first to avoid partial matches.
Example: `Twelve Angry Men → 12 Angry Men`, `Second Chance → 2 Chance`.

**Trailing article normalization:** Move a leading definite/indefinite article to the
end, comma-separated. Articles: `The`, `A`, `An`.
Example: `The Addams Family → Addams Family, The`, `A Dog → Dog, A`.
Inverse is also needed for display: `Addams Family, The → The Addams Family`.
This matches the sheet's storage convention.

**Signal extraction:** Extract `(name, manufacturer, year)` from a VPX filename stem.
Pattern: `^(.*?)\s*\(\s*([^\d(),]+)\s*,?\s*(\d{4})` (case-insensitive).
Both `Name (Manufacturer Year)` and `Name (Manufacturer, Year)` are supported.
Trailing noise (`v600`, `full dmd`, ` 1.0`) after the closing parenthesis is discarded.
Replace underscores with spaces before matching.
Examples:
```
"Phoenix (Williams 1978) v600"       → ("Phoenix", "Williams", 1978)
"Firepower (Williams 1980) full dmd" → ("Firepower", "Williams", 1980)
"Flash (Williams, 1979)"             → ("Flash", "Williams", 1979)
```
Returns nil/null when no structured signal is found.

**Full normalization for fallback matching:**
When no structured signal is found, normalize for full-catalog fuzzy matching:
1. Apply all norm passes (norms, trailing article).
2. Replace underscores and hyphens with spaces.
3. Strip noise words: version strings (`v600`, `1.0`), format tags (`vpx`, `vpt`),
   qualifiers (`mod`, `update`, `final`, `beta`, `alpha`, `release`, `rc1`).
Pattern: `\b(v?\d+[\d.]*|vpx|vp[xt]?|mod|update|final|beta|alpha|release|rc\d*)\b` (case-insensitive).
4. Collapse whitespace.

---

### 9.7 Fuzzy Matching Engine

**Algorithm:** RapidFuzz WRatio (combination of ratio, partial ratio, token sort ratio,
token set ratio) — provides the best balance for messy filename stems.

**Two-path matching:**

*Path A — same-era (preferred):*
Used when `extract_signal` returns a result from the query stem.
1. Filter catalog to entries where `manufacturer` matches (case-insensitive) and
   `year` matches exactly.
2. Normalize both the query name and each entry's `name` field using `_norm_name`:
   apply all norm passes, apply trailing article, lowercase, replace non-alphanumeric
   runs with spaces.
3. Score each candidate with WRatio.
4. Sort descending by score.
5. **Only use same-era results if the best score ≥ interactive threshold (72).**
   If the best same-era score is below 72, the manufacturer/year in the filename is
   probably wrong — fall through to Path B.
6. Return top `limit` (default 5) results.

*Path B — full catalog fallback:*
Used when no signal is found, or when same-era filtering found nothing above 72.
1. Normalize the query stem using `normalise_for_matching` (full noise-stripping).
2. Normalize each catalog entry's full `game_name` the same way.
3. Score against all entries with WRatio.
4. Return top `limit` results.

**Thresholds:**
- `MATCH_THRESHOLD_AUTO = 92`: auto-assign without prompting.
- `MATCH_THRESHOLD_INTERACTIVE = 72`: offer as candidate in interactive mode.
- Below 72: sent to review (non-interactive) or user decides (interactive).

**Force-match by ID:** In interactive mode, the user can type a `MasterID` (e.g.
`VPW-12345`) or an `IPDBNum` (e.g. `5678`). Exact case-insensitive string match on
the respective field; confidence = 100; `match_field = "master_id"` or `"ipdb_num"`.

---

### 9.8 PUPDatabase (SQLite)

**File:** `PUPDatabase.db` at the configured path.

**Tables touched:**
- `Emulators` — read only. Never write. Query to find the VPX emulator row.
- `Games` — read and write. The only table this tool modifies.
- `GameScan` — **never touch.** PinUP Popper owns this table; writes corrupt scan state.

**VPX emulator lookup:**
```sql
SELECT EMUID FROM Emulators WHERE EmuName LIKE '%Visual Pinball X%' LIMIT 1
```
Returns `null` if not found; this causes `REGISTER_GAME` actions to fail gracefully.

**Game upsert logic:**
1. Check if a row with matching `GameFileName` (case-insensitive) already exists in `Games`.
2. If found: UPDATE that row. Include `DateUpdated = now`.
3. If not found: INSERT a new row. Include `DateAdded = now`.

**Fields written to `Games`:**

| Column | Value | Notes |
|--------|-------|-------|
| `EMUID` | VPX emulator ID | From Emulators lookup |
| `GameName` | `canonical_filename` | e.g. `PIN-BOT (Williams, 1986)` |
| `GameFileName` | bare filename | e.g. `PIN-BOT (Williams, 1986).vpx` |
| `GameDisplay` | `entry.name` | Title only, without manufacturer/year |
| `Visible` | `1` | Always 1 for newly installed tables |
| `GameYear` | `entry.year` | Integer |
| `Manufact` | `entry.manufacturer` | |
| `GameType` | `entry.game_type` or `""` | EM / SS / DMD / etc. |
| `DateFileUpdated` | file mtime or now | Format: `YYYY-MM-DD HH:MM:SS.mmm` |
| `WEBGameID` | `entry.master_id` | Stable catalog key |
| `TAGS` | formatted tag string | See below; omitted if no tags |
| `IPDBNum` | `entry.ipdb_num` | Omitted if null |
| `WebLinkURL` | `entry.ipdb_url` | IPDB URL; omitted if null |
| `WebLink2URL` | `entry.vps_link` | VPS spreadsheet link; omitted if null |
| `DesignedBy` | `entry.designed_by` | Omitted if null |
| `Notes` | `entry.notes` | Omitted if null |
| `DateAdded` | now | Only on INSERT |
| `DateUpdated` | now | Only on UPDATE |

**TAGS format:** Comma-separated double-quoted values. Each value has internal double
quotes backslash-escaped. Tag list: `["VPW"]` if `vpw_link` is set; `[entry.tier]`
if tier is non-empty. Combined: `"VPW","Tier Name"`.

**Backup convention:** Filename `{YYYYMMDDHHMMSS}_PUPDatabase.db` in `PUPBackup/`.
Timestamp-first so files sort chronologically. One backup per session (first write
triggers it; subsequent writes in the same session reuse it). Rotation: 5 backups
per calendar day, 5 days retained.

---

## 10. Milestones

### Milestone 1 — Infrastructure and TUI skeleton

Deliverable: A runnable Go binary with CLI argument parsing and a persistent terminal
status bar that can display phase labels and a spinner. No business logic.

Requirements:
- CLI: `vpin process [--db PATH] [--dir PATH] [--auto] [--dry-run] [--rehearsal]`
- Persistent status bar always visible at the bottom of the terminal, showing phase name
  and spinner. Phases: IDLE, LOADING, SCANNING, MATCHING, EXECUTING, DONE.
- Arrow-key picker widget that blocks and returns a selected value. Renders above the
  status bar as a conditional panel (not a separate process).
- Single-line text input widget with the same lifetime as the picker.
- `tui.Print(richMarkup)` emits output above the status bar.
- Headless fallback (non-TTY): no TUI, all output to stdout.

Verification: Run `vpin process`; see spinner, type arrows, pick an option, see
confirmation echoed, status bar stays pinned throughout.

---

### Milestone 2 — File and archive classification in isolation

Deliverable: A standalone classification library with no UI or catalog dependency.
Tested by unit tests against fixture files and member lists.

Requirements:
- `classify_file(path, peekZip)` → `FileTypeInfo{Category, Description}`.
- `classifyDirectory(name, memberNames)` → `FileTypeInfo | nil`.
- `classifyMembers(memberList)` → `[]MemberGroup{Representative, Members, Info, IsBundle}`.
- ROM zip detection (content inspection logic in Section 3.4).
- POV ini detection (`is_pov_ini` — content inspection in Section 3.3).
- Peek for `.zip`, `.7z`, `.rar` (member list only, no extraction).
- Extract for `.zip`, `.7z`, `.rar` to a destination directory.

Verification: Unit tests cover every asset type's detection logic; ROM vs distribution
zip; bundle directory detection for all six bundle types; archive member grouping with
dissolved non-bundle directories.

---

### Milestone 3 — Catalog and matching engine in isolation

Deliverable: A catalog library that loads the xlsx, parses it, and runs fuzzy matching.
No UI dependency. Tested against known filename/catalog pairs.

Requirements:
- Download xlsx from Google Sheets URL.
- Parse all tabs with a `GameName` column (dynamic tab discovery, not hardcoded names).
- `SheetEntry` struct with all fields and `canonical_filename` property.
- Normalization pipeline: possessives, CamelCase, Roman numerals, number words,
  trailing article. Each pass independently testable. Applied before scoring.
- `extract_signal` — parse `(name, manufacturer, year)` from a stem.
- `find_match(stem, catalog, limit)` → `[]MatchResult` — same-era path + full-catalog
  fallback, as specified in Section 9.7.
- `best_match(stem, catalog, threshold)` → `MatchResult | nil`.
- Force-match by MasterID and IPDBNum.
- Catalog age check (file mtime comparison).

Verification: Test suite with known-good `(filename, expected_canonical)` pairs
covering same-era matching, Roman numeral normalization, CamelCase splitting, trailing
articles, and full-catalog fallback. All should produce ≥92% confidence.

---

### Milestone 4 — Plan builder (scan, classify, match, deduplicate)

Deliverable: `build_plan(downloadsDir, interactive, catalog)` returns a complete
`ProcessPlan` without touching the filesystem. The plan is fully testable by
inspecting action types and destinations.

Requirements:
- Two-pass scan: bundle directory pre-pass, then recursive file walk.
- Per-file and per-archive planning using Milestone 2 classification and Milestone 3 matching.
- `PlannedAction` tree with `EXTRACT_ARCHIVE` → `[VAULT_ARCHIVE, ...member actions]`.
- `REGISTER_GAME` back-pointer on `MOVE_VPX` for dedup.
- Virtual path tracking for archive members.
- Deduplication post-pass: confidence + mtime ranking, loser rerouting to review,
  back-pointer kill of orphaned REGISTER_GAME.
- Interactive prompt callback (user picks match, ignores, reviews, or force-searches).
- Dry-run report: tree-format plan with action labels, summary counts table.
- Rehearsal path remapping: `_remap_plan_dests`.

Verification: Port the existing test suite (65 tests, 3 xfailed) for Williams 1970s
and 1980s tables. All passing tests should pass; the 3 xfailed tests document known
NAMING edge cases and should remain marked as expected failures.

---

### Milestone 5 — Execution and DB integration

Deliverable: `execute_plan(plan, vpxEmuid)` walks the plan tree and performs all
filesystem mutations and DB writes.

Requirements:
- Extract archives to named subdirectories; vault originals to `archive_vault/`.
- Rename and move all asset types per plan.
- Move review items to `review/`; move ignored items to `ignored/`.
- Nested archive recursive extraction (post-member-processing scan).
- Cleanup extracted directory after all members are moved.
- `upsert_game` — insert or update `Games` row with all fields from Section 9.8.
- Backup-before-first-write: one backup per session, rotation policy enforced.
- `REVIEW.md` append with timestamped markdown table.
- Per-action failure reporting; execution continues on failure.
- Post-execution summary: review bullets, installed tables list, type counts.

Verification: Rehearsal mode against a fixture downloads directory produces correct
file layout in `rehearsal/` and correct DB state in `rehearsal/PUPDatabase.db`.
No writes to real install paths.

---

### Milestone 6 — Complete binary: interactive UX, all CLI modes

Deliverable: The complete `vpin` binary with all three modes and full interactive UX.

Requirements for `process` mode:
- Full interactive matching flow: confidence-bar choice list, force-search by
  name/MasterID/IPDB#, send-to-review, ignore, quit.
- Catalog freshness prompt (missing → default Yes; stale → default No).
- Confirmation prompt before execution in interactive mode.
- Status bar progress during scanning (done/total, per-type counts as they accumulate).
- `--rehearsal` flag with wipe-and-rebuild confirmation.
- `--auto` flag disables all interactive prompts.

Requirements for `monitor` mode:
- Load catalog and compare to Games table.
- Report: tables in catalog but not installed; tables installed but not in catalog;
  GameName mismatches (installed filename matches catalog but display name differs).

Requirements for `download` mode:
- Find tables in catalog with no matching installed table.
- Group by manufacturer and decade.
- Identify download URLs from `VPW Version Link` and `VPS Link` fields.
- Open URLs in the system browser in batches.

Verification: Full end-to-end run in rehearsal mode with a representative downloads
directory containing loose files, archives, nested archives, bundle directories, ROM
zips, and duplicate-destination conflicts. All 65 passing tests still pass.

---

## Validation Checklist

- [x] Every asset type appears in Section 3 (11 content types + distribution archives)
- [x] Every archive format appears in Section 4 (ZIP, 7z, RAR)
- [x] Section 9 is precise enough to implement without the original code
- [x] Milestones are in dependency order (infra → classification → catalog → plan → execute → UX)
- [x] No section uses a class name or function name as its explanation
