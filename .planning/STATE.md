---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: in_progress
stopped_at: Completed 07-03-PLAN.md (GitHub Actions ci.yml, release.yml, check-docs.sh)
last_updated: "2026-05-16T16:22:00.000Z"
progress:
  total_phases: 7
  completed_phases: 5
  total_plans: 39
  completed_plans: 36
  percent: 92
---

# State: vpin-plunger

## Project Reference

**Core Value:** Scan downloads, plan every move, show the plan — then execute only when the user says go.

**What This Is:** Go rewrite of vPinManager — headless TUI CLI for virtual pinball cabinet operators. Scans a flat `downloads/` directory, classifies every asset by type, renames to canonical `{Name} ({Manufacturer}, {Year})` convention via fuzzy match against the community catalog, moves files to correct VPX install paths, and registers new tables in PinUP Popper's SQLite database — all after the user reviews and confirms a dry-run plan.

**Repository:** https://github.com/iainconnor/vpin-plunger

---

## Current Position

**Active Phase:** Phase 7 — Documentation & CI/CD
**Active Plan:** 07-03 complete
**Phase Status:** In progress (3/4 plans complete)
**Overall Progress:** 6 phases complete + Phase 7 in progress

```
Phase 1 [##########] 100% TUI Skeleton ✓
Phase 2 [##########] 100% Classification & Archive Handling ✓
Phase 3 [##########] 100% Catalog & Matching Engine ✓
Phase 4 [##########] 100% Plan Builder ✓
Phase 5 [##########] 100% Executor & Database ✓
Phase 6 [##########] 100% Full Binary ✓
Phase 7 [######    ]  75% Documentation & CI/CD (3/4 plans)
```

---

## Phase Summary

| Phase | Name | Requirements | Status |
|-------|------|-------------|--------|
| 1 | TUI Skeleton | TUI-01 → TUI-10 (10 req) | Complete ✓ |
| 2 | Classification & Archive Handling | CLF-01 → CLF-11, ARC-01 → ARC-07 (18 req) | Complete ✓ |
| 3 | Catalog & Matching Engine | CAT-01 → CAT-09 (9 req) | Complete ✓ |
| 4 | Plan Builder | PLN-01 → PLN-09 (9 req) | Complete ✓ |
| 5 | Executor & Database | EXE-01 → EXE-09 (9 req) | Not started |
| 6 | Full Binary | MOD-01 → MOD-08 (8 req) | Complete ✓ |

---

## Performance Metrics

**Plans completed:** 19
**Plans total:** TBD (derived per phase during planning)
**Requirements delivered:** 37/63 (CAT-01 through CAT-09 all complete)
**Phases complete:** 3/6

---

## Accumulated Context

### Key Decisions Logged

| Decision | Rationale |
|---|---|
| Go rewrite (not Python continuation) | Single native binary, CGO_ENABLED=0, better cross-platform TUI |
| bubbletea v2 strict Elm | Enforces testable state, eliminates race conditions, makes concurrency explicit |
| mholt/archives for Walk() | Unified FS abstraction over ZIP/7z/RAR/tar; single dependency |
| formats/ has zero UI dependency | Enables unit testing classification logic without TUI |
| Plan-then-confirm UX | Dry-run by default; execution requires explicit approval |
| Catalog staleness: 7 days | 60-second threshold in Python was too aggressive for infrequently-updated sheet |
| Pure-Go SQLite driver | CGO_ENABLED=0 rules out mattn/go-sqlite3; modernc.org/sqlite preferred |
| 12 per-asset insert colors | Each asset type needs distinct visual identity for pill badges in TUI |
| applyTrailingArticle uses 3-capture regex | Preserves (Manufacturer, Year) block position; article inserted before metadata, not at end |
| catalog.Config standalone (no internal/config import) | Prevents circular imports; app/ maps CLI flags to catalog.Config in Phase 6 wiring |
| Load() and match methods are Wave 1 skeletons | Locks public API for Wave 2 to implement; compile-clean with errNotImplemented sentinel |
| FindMatch/BestMatch/ForceMatch implemented in match.go (plan 03-05) | catalog.go owns struct+Load; match.go owns all three matching methods; CAT-05/06/07 delivered |
| rows.Close() called explicitly per sheet (not deferred) | Surfaces close error per sheet; releases temp XML files from excelize (RESEARCH.md Pitfall 2) |
| Download uses io.LimitReader 50 MB cap | Guards against oversized/malicious xlsx response (security domain) |
| TestNormalizeForMatching uses invariant check (len-reduction) not exact equality | Tolerates case-preservation vs lowercase differences; noise-stripping contract is removal of version tokens, not case normalization |
| naming_test.go uses white-box package naming declaration | Enables testing of unexported helpers (stripPossessive, splitCamelCase, etc.) per Phase 2 convention |
| planner.Config standalone (no internal/config import) | Prevents circular imports; mirrors catalog.Config pattern; Phase 6 constructs from CLI flags |
| ActionType uses iota int (not string) | Zero value ActionTypeUnknown signals coding error; String() method for display labels |
| MatchChoice four-arm struct (Match/ForceID/SendToReview/Ignore) | Decouples planner from TUI picker; BuildPlan unaware of bubbletea; Phase 6 wires channel pair |
| BuildPlan stub returns empty ProcessPlan in Wave 1 | Locks public API for Wave 2; full implementation assembled in 04-02 (scan.go) and 04-03 (match.go) |
| bundlePrePass claims dirs before Walk so Pass 2 returns fs.SkipDir | Prevents double-classification of bundle members (PLN-01) |
| ROM zip detection .zip only | ROM archives are always ZIP; .7z/.rar always treated as distribution archives |
| Archive member mtime = parent archive mtime | Member mtimes unavailable at plan time (RESEARCH Pitfall 2) |
| Skip go mod tidy after go get (Wave 0) | tidy prunes unimported deps; Wave 1 db.go blank import will anchor modernc.org/sqlite permanently |
| GameRecord is plain value struct, no planner import (D-13) | Executor maps PlannedAction fields to GameRecord before calling UpsertGame; db/ stays circular-import-free |
| db.validateColumns closes rows explicitly before lookupEMUID | Avoids database lock (modernc.org/sqlite WAL mode); RESEARCH Pitfall 4 |
| pruneBackups uses lexicographic sort of timestamp-first filenames | YYYYMMDDHHMMSS prefix means string sort = chronological sort; no time.Parse needed |
| executor.Config standalone (no internal/config/ import) | Prevents circular imports; mirrors planner.Config and catalog.Config pattern (D-04) |
| detectHandler uses inline []formats.Handler slice | No RegisteredHandlers() function exists; inline is the only safe approach |
| IGNORE arm always appends reviewEntry (D-07) | REVIEW.md includes ALL items needing human attention: review-routed, ignored, and failed |
| executeAction walks tree by action.Type only (RESEARCH Pitfall 6) | RegisterGame back-pointer is for dedup post-pass only; executor ignores it |
| newDB constructor wraps existing *sql.DB for test injection | Allows openTestDB to pre-populate :memory: schema before validateColumns runs; Open delegates to newDB |
| tea.KeyPressMsg{Code: tea.KeyEnter} is correct bubbletea v2 literal | KeyMsg is an interface in v2; KeyPressMsg is the concrete struct; plan spec used obsolete v1 syntax |
| advanceToCatalogFreshCheck helper in update.go for rehearsal-wipe chaining | Rehearsal-wipe Yes path chains into catalog freshness check; helper re-seeds Picker from m.Catalog metadata |
| internal/config package is the single CLI-flag-to-Config translation layer | Domain packages (planner/executor/catalog) remain standalone; only cmd/plunger/*.go imports config/; prevents circular imports (Pitfall 7) |
| ValidatePaths checks 11 content dirs only; operational dirs excluded | Vault/review/ignored/rehearsal dirs are created on-demand by executor; no need to pre-validate (D-14) |
| AllGames uses two const SQL strings (filtered/no-filter) | Avoids dynamic string-building; consistent with UpsertGame bind-parameter pattern; EMUID filter applied at query level not in Go |
| createSchema fixture seeds a VPX emulator row | openTestDB produces a valid emuid for all EMUID-filter-dependent tests; all existing tests unaffected (UpsertGame uses d.emuid implicitly) |
| DetectCatalogFreshness uses two-pass catalog.IsStale | Huge threshold (~146yr) distinguishes missing from stale; configured threshold then checks actual staleness |
| CachePath() and Staleness() accessors on catalog.Catalog | Enables app/ to call package-level IsStale without accessing private cfg; maintains no-circular-import invariant |
| --auto collapses both pre-scan pickers to non-interactive paths | StateRehearsalWipeCheck → os.RemoveAll; StateCatalogFreshCheck → immediate StartProcess(cat) |
| monitor.go defers main.go registration to Plan 07 | Plan 07 owns cmd/plunger/main.go as the single shared file; avoids conflicting edits across plans |
| monitor non-TTY uses tea.WithoutRenderer without --auto gate | Monitor has no pre-scan pickers; any invocation is safely pipeable regardless of TTY state |
| Windows openURL uses exec.Command("cmd", "/c", "start", "", url) with empty string before URL | Prevents cmd.exe treating URL as window title when URL contains & (RESEARCH Pitfall 6); arg-list form prevents shell injection |
| goreleaser-action@v7 with args: release --clean | --rm-dist removed in goreleaser v2; --clean is correct; version: ~> v2 pins to latest v2.x patch |
| release.yml requires fetch-depth: 0 and permissions: contents: write | goreleaser needs full git history for changelog; needs contents: write to create GitHub Release (without it: HTTP 403) |
| doc-freshness CI job pinned to ubuntu-latest | check-docs.sh uses GNU grep -oP (Perl regex); macOS runners ship BSD grep which lacks -P |
| check-docs.sh is self-locating via BASH_SOURCE | Script can be invoked from any CWD; REPO_ROOT computed relative to script location, not caller CWD |
| README.md written with all 11 sections per CONTEXT.md D-01 through D-06 | All 16 process flags, 2 monitor flags, 3 download flags documented; ASCII plunger hero; locked tagline; Baller Installer default paths table; confidence thresholds explained |
| downloadBuildCmd refactored to delegate to buildDownloadGroups private helper | DownloadBuildOnce and downloadBuildCmd share one code path; post-TUI URL open uses identical data path as tea.Cmd pipeline |
| auto-wipe rehearsal/ re-seeds DB after wipe | runProcess pre-copies DB before wipe check; wipe deletes copy; fix re-copies original DB so executor gets valid schema (Rule 1 bug fix) |
| TestE2ERehearsalAuto uses Phoenix (Williams, 1978).vpx fixture | Exactly matches catalog entry → 100% confidence → ActionTypeMoveVPX → file lands in rehearsal/ not review/ |

### Critical Constraints (Never Violate)

- `CGO_ENABLED=0` always — no C dependencies
- No raw goroutines — all concurrency via `tea.Cmd`
- No logic in `View()` — pure render only
- All `lipgloss.Color` values in `internal/ui/theme/` only
- `formats/` package: zero imports from `ui/` or `catalog/`
- `main.go`: wiring only, no domain logic
- All new dependencies must be flagged before adding (`make tidy` gate)

### Domain Facts

- Canonical filename: `{Name} ({Manufacturer}, {Year})` with trailing-article convention (e.g. "Addams Family, The")
- 11 content asset types + 1 delivery type (distribution archive)
- Confidence thresholds: auto-assign ≥92%, interactive prompt 72–91%, review <72%
- Catalog: Google Sheet ID `12-Pwub-p4krv17cwOFT4kr4qR_a34B_YbBvHjEcpSI0`
- VPX install layout: Baller Installer standard at `C:\vPinball\visualpinball\`
- ROM zips are never extracted — install verbatim
- PuP packs matched by exact directory name — no catalog lookup
- 65 Python tests must port and pass; 3 xfailed naming-edge-cases remain expected-failure

### Testing Expectations Per Phase

| Phase | Minimum Test Coverage |
|---|---|
| 1 | Smoke test: binary starts, status bar visible in View(), WindowSizeMsg no panic |
| 2 | Unit test every classifier rule; Peek + Extract with fixture ZIP and 7z |
| 3 | Each normalization pass independently tested; ≥10 known filename→canonical pairs |
| 4 | All 65 ported Python tests pass; 3 xfailed remain marked expected-failure |
| 5 | Rehearsal-mode smoke test against fixture downloads/; DB upsert idempotency test |
| 6 | End-to-end rehearsal run: `vpin process --rehearsal --auto` exits 0 with expected output |

### Skeleton State at Roadmap Creation

| File | Status |
|---|---|
| `cmd/plunger/main.go` | Skeleton — wires bubbletea, passes nil model |
| `internal/ui/theme/colors.go` | Partial — base palette present; insert colors pending (Phase 1) |
| `internal/ui/components/statusbar.go` | Skeleton — View renders bar; Init/Update stubs |
| `internal/ui/components/progress.go` | Skeleton — types declared, not implemented |
| `internal/formats/format.go` | Skeleton — interface declared; Walk() wired to mholt/archives |
| `go.mod` | Incomplete — no require block; `go mod tidy` needed in Phase 1 |

### Todos

- (none yet — populate during phase planning)

### Blockers

- (none)

---

## Session Continuity

**Last session:** 2026-05-16T16:22:00Z
**Stopped at:** Completed 07-03-PLAN.md (GitHub Actions ci.yml, release.yml, check-docs.sh)
**Next action:** Execute 07-04-PLAN.md — CONTRIBUTING.md

---

*State initialized: 2026-05-09*
*Last updated: 2026-05-10 after Phase 3 context session*
