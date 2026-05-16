# Roadmap: vpin-plunger

**Core Value:** Scan downloads, plan every move, show the plan — then execute only when the user says go.

**Granularity:** Standard (5-8 phases)

**Coverage:** 63/63 v1 requirements mapped

---

## Phases

- [x] **Phase 1: TUI Skeleton** — Bubbletea Elm scaffold with pinned status bar, picker, text input, all 12 insert colors, and CLI flag wiring
- [x] **Phase 2: Classification & Archive Handling** — Complete asset classifier and ZIP/7z/RAR peek-and-extract with unit tests
- [x] **Phase 3: Catalog & Matching Engine** — Google Sheets xlsx download, normalization pipeline, and fuzzy matching with confidence thresholds
- [x] **Phase 4: Plan Builder** — Two-pass scan, ProcessPlan tree, deduplication, dry-run report, rehearsal remapping, and all 65 ported Python tests (completed 2026-05-14)
- [x] **Phase 5: Executor & Database** — File moves, archive extraction, PUPDatabase upsert, backup rotation, REVIEW.md, and per-action failure handling (completed 2026-05-14)
- [x] **Phase 6: Full Binary** — Complete process/monitor/download CLI modes wired end-to-end with full-run rehearsal smoke test (completed 2026-05-16)
- [ ] **Phase 7: Documentation & CI/CD** — Beautiful README with branding, full CLI reference, build/install guide, GitHub Actions pipeline, release automation, and doc-freshness hooks

---

## Phase Details

### Phase 1: TUI Skeleton
**Goal**: A working bubbletea binary the operator can launch that displays the full visual chrome — pinned status bar, picker widget, text input, progress indicators, and all 12 per-asset insert colors — and responds correctly to terminal resize and non-TTY environments.
**Depends on**: Nothing (first phase)
**Requirements**: TUI-01, TUI-02, TUI-03, TUI-04, TUI-05, TUI-06, TUI-07, TUI-08, TUI-09, TUI-10
**Success Criteria** (what must be TRUE):
  1. `vpin process` starts, shows the DMD amber status bar pinned at the bottom, and exits cleanly — binary is launchable
  2. Arrow-key picker widget renders above the status bar with selectable rows; text input captures keystrokes without corrupting the bar
  3. Terminal resize at any moment does not produce visual corruption or layout breakage
  4. Running `vpin process > /dev/null` (non-TTY) emits plain stdout with no TUI escape codes
  5. `go test ./internal/ui/...` passes: smoke test asserts View() renders a string containing the status bar and that WindowSizeMsg is handled without panic
**Plans** (7 plans):
- [x] 01-01-PLAN.md — Insert color constants + go mod tidy (TUI-07)
- [x] 01-02-PLAN.md — Progress components: DeterminateBar + IndeterminateSpinner (TUI-09, TUI-10)
- [x] 01-03-PLAN.md — StatusBar 4-zone layout with pill renderer (TUI-01)
- [x] 01-04-PLAN.md — Picker widget with text input and action shortcuts (TUI-02, TUI-03)
- [x] 01-05-PLAN.md — internal/app package: Model + Update + View + Print (TUI-04, TUI-06)
- [x] 01-06-PLAN.md — cmd/plunger CLI: cobra subcommand + non-TTY detection (TUI-05, TUI-08)
- [x] 01-07-PLAN.md — Smoke tests for ui/components and app packages (go test ./internal/ui/...)
**UI hint**: yes

### Phase 2: Classification & Archive Handling
**Goal**: The formats/ package correctly identifies every asset type from extensions, content inspection, and directory signals, and can peek or extract ZIP/7z/RAR archives — all independently testable with no TUI dependency.
**Depends on**: Phase 1
**Requirements**: CLF-01, CLF-02, CLF-03, CLF-04, CLF-05, CLF-06, CLF-07, CLF-08, CLF-09, CLF-10, CLF-11, ARC-01, ARC-02, ARC-03, ARC-04, ARC-05, ARC-06, ARC-07
**Success Criteria** (what must be TRUE):
  1. `ClassifyFile` returns the correct AssetType for every extension covered by CLF-01 through CLF-03 (VPX, backglass, NVRAM, POV ini, ROM zip)
  2. `ClassifyDirectory` identifies Music, Altcolor, Altsound, UltraDMD, FlexDMD, and PuP pack directories from their marker files/naming rules
  3. `ClassifyMembers` groups bundle directories as units and dissolves unrecognised directories to individual files; files inside bundles are never re-classified
  4. `ZIPHandler.Peek`, `SevenZipHandler.Peek`, and `RARHandler.Peek` return correct member lists against fixture archives without extracting; `Extract` delivers files to destination with zip-slip protection
  5. `go test ./internal/formats/...` passes: one test case per classifier rule, two fixture archives (ZIP and 7z) exercising Peek and Extract
**Plans** (3 plans):
- [x] 02-01-PLAN.md — AssetType enum + Handler interface + classify.go (CLF-01..CLF-11)
- [x] 02-02-PLAN.md — ZIP/7z/RAR handlers + testdata fixtures (ARC-01..ARC-06)
- [x] 02-03-PLAN.md — Walk replacement with fs.DirEntry callback + Walk tests (ARC-07)

### Phase 3: Catalog & Matching Engine
**Goal**: The catalog package downloads, parses, and fuzzy-matches filenames against the community Google Sheet, applying the full normalization pipeline, confidence thresholds, and force-match paths — all independently unit-testable.
**Depends on**: Phase 2
**Requirements**: CAT-01, CAT-02, CAT-03, CAT-04, CAT-05, CAT-06, CAT-07, CAT-08, CAT-09
**Success Criteria** (what must be TRUE):
  1. Catalog downloads the xlsx when cache is absent or stale, skips download when fresh, and holds the parsed entries in an in-process cache for the lifetime of the run (never re-parsed per query)
  2. `SheetEntry.CanonicalFilename` returns `{Name} ({Manufacturer}, {Year})` with trailing-article convention for every entry
  3. The normalization pipeline transforms known noisy filenames to their canonical equivalents: possessives stripped, CamelCase split, Roman numerals II–XX expanded, number words 0–12 converted, trailing articles moved
  4. `BestMatch` auto-assigns at ≥92% confidence, prompts at 72–91%, and returns nil below 72%; force-match by MasterID or IPDBNum returns confidence=100
  5. `go test ./internal/catalog/...` passes: each normalization pass tested independently; ≥10 known (filename → canonical) pairs covering edge cases from the migration brief
**Plans** (7 plans):
- [x] 03-01-PLAN.md — Wave 0: add go-fuzzywuzzy + excelize via go get + go mod tidy (CAT-01, CAT-02, CAT-05)
- [x] 03-02-PLAN.md — internal/naming/ package: normalization pipeline + Canonical + ExtractSignal (CAT-03, CAT-04, CAT-08)
- [x] 03-03-PLAN.md — catalog types + Config + Catalog struct skeleton with Load/match method declarations (CAT-03, CAT-09)
- [x] 03-04-PLAN.md — download.go + parse.go + Load() body (CAT-01, CAT-02, CAT-03, CAT-09)
- [x] 03-05-PLAN.md — match.go: Path A + Path B + BestMatch threshold gate + ForceMatch (CAT-05, CAT-06, CAT-07)
- [x] 03-06-PLAN.md — internal/naming/naming_test.go: per-pass + full-pipeline + signal + canonical tests (CAT-04, CAT-08)
- [x] 03-07-PLAN.md — internal/catalog/catalog_test.go: TestMain fixture + httptest + matching tests (CAT-01, CAT-02, CAT-03, CAT-05, CAT-06, CAT-07, CAT-09)

### Phase 4: Plan Builder
**Goal**: The planner package produces a complete, correct ProcessPlan tree from a downloads/ directory — including deduplication, dry-run formatting, rehearsal remapping, and the full interactive prompt callback — and all 65 ported Python tests pass.
**Depends on**: Phase 3
**Requirements**: PLN-01, PLN-02, PLN-03, PLN-04, PLN-05, PLN-06, PLN-07, PLN-08, PLN-09
**Success Criteria** (what must be TRUE):
  1. `BuildPlan` runs a bundle pre-pass before the recursive walk so bundle members never appear as standalone file actions
  2. The ProcessPlan tree for an archive input contains EXTRACT_ARCHIVE with VAULT_ARCHIVE and typed member actions as children; MOVE_VPX actions carry a back-pointer to their REGISTER_GAME action
  3. Deduplication correctly routes losing MOVE_VPX actions to review and nullifies their paired REGISTER_GAME via the back-pointer
  4. Dry-run report prints a tree-format plan with action labels, virtual paths (e.g. `archive.zip/game.vpx`), and a summary counts table; `--rehearsal` rewrites all destination paths to `rehearsal/`
  5. `go test ./internal/planner/...` passes: all 65 ported Python tests pass; 3 xfailed naming-edge-case tests remain marked as expected failures
**Plans** (6 plans):
- [x] 04-01-PLAN.md — ActionType enum + PlannedAction + ProcessPlan types + Config struct (PLN-02, PLN-03, PLN-05, PLN-08)
- [x] 04-02-PLAN.md — Two-pass scan: bundle pre-pass + recursive Walk + archive action trees (PLN-01, PLN-02, PLN-05)
- [x] 04-03-PLAN.md — Match integration: BestMatch auto-assign + matchFn callback + BuildPlan body (PLN-08, PLN-01, PLN-02, PLN-03)
- [x] 04-04-PLAN.md — Deduplication post-pass with back-pointer RegisterGame kill (PLN-03, PLN-04)
- [x] 04-05-PLAN.md — Rehearsal path remapping + FormatPlan dry-run formatter (PLN-06, PLN-07)
- [x] 04-06-PLAN.md — Full test suite: TestMain fixture + ~65 test cases + 3 xfail (PLN-09)

### Phase 5: Executor & Database
**Goal**: The executor and db packages perform every filesystem mutation and database write described in the plan, with backup rotation, REVIEW.md logging, and per-action failure tolerance — validated by a rehearsal-mode smoke test against a fixture downloads/ directory.
**Depends on**: Phase 4
**Requirements**: EXE-01, EXE-02, EXE-03, EXE-04, EXE-05, EXE-06, EXE-07, EXE-08, EXE-09
**Success Criteria** (what must be TRUE):
  1. `ExecutePlan` in rehearsal mode against a fixture downloads/ directory moves all fixture files to `rehearsal/` subdirectory equivalents without touching the originals; archive vault is created under `rehearsal/archive_vault/`
  2. Review items land in `review/`; ignored items land in `ignored/`; `review/REVIEW.md` is appended with a timestamped markdown table (never replaced)
  3. `UpsertGame` inserts a `Games` row into a fixture PUPDatabase; a second run updates (not duplicates) the row; `GameScan` table is untouched
  4. PUPDatabase is backed up before the first write; a second session produces a second backup; rotation keeps at most 5 backups/day × 5 days
  5. A single action failure (e.g. permission denied on one file) is logged but execution continues and remaining actions complete; `go test ./internal/executor/... ./internal/db/...` passes with fixture-based cases
**Plans** (5 plans):\n- [x] 05-00-PLAN.md � Add modernc.org/sqlite dependency (EXE-05, EXE-06, EXE-07)\n- [x] 05-01-PLAN.md � internal/db/: DB struct, Open, UpsertGame, backup rotation (EXE-05, EXE-06, EXE-07)\n- [x] 05-02-PLAN.md � internal/executor/: Config, ExecutePlan, move helpers, REVIEW.md (EXE-01..EXE-04, EXE-08, EXE-09)\n- [x] 05-03-PLAN.md � internal/db/db_test.go: UpsertGame idempotency, column validation, backup tests (EXE-05..EXE-07)\n- [ ] 05-04-PLAN.md � internal/executor/executor_test.go: TestMain fixture, smoke tests (EXE-01..EXE-04, EXE-08, EXE-09)

### Phase 6: Full Binary
**Goal**: All three CLI modes (process, monitor, download) are wired end-to-end through the bubbletea app layer; the binary passes a full rehearsal run against a representative fixture set and every integration path is exercised.
**Depends on**: Phase 5
**Requirements**: MOD-01, MOD-02, MOD-03, MOD-04, MOD-05, MOD-06, MOD-07, MOD-08
**Success Criteria** (what must be TRUE):
  1. `vpin process --rehearsal` against a representative fixture downloads/ directory completes a full run: catalog load prompt → scan → interactive match → dry-run display → confirmation → execution → DONE — with all output appearing above the pinned status bar
  2. `vpin process --auto --rehearsal` runs non-interactively: unmatched files go to review, no prompts appear, execution completes without human input
  3. `vpin monitor` loads the catalog, compares against a fixture Games table, and reports not-installed, not-in-catalog, and GameName-mismatch entries to stdout
  4. `vpin download` identifies uninstalled entries from the fixture catalog, groups them by manufacturer and decade, and opens (or prints, in dry-run) the correct VPW/VPS URLs
  5. End-to-end rehearsal smoke test in CI: `vpin process --rehearsal --auto` exits 0, fixture rehearsal/ directory contains expected renamed files, REVIEW.md exists if any review items were produced
**Plans** (7 plans):
- [x] 06-01-PLAN.md - app/ messages.go + cmds.go + Model/Update extensions (MOD-01, MOD-03, MOD-04, MOD-06)
- [x] 06-02-PLAN.md - internal/config/ package: path defaults + Build*Config + ValidatePaths (MOD-02, MOD-05)
- [x] 06-03-PLAN.md - internal/db/ AllGames() + GameRow + tests (MOD-07)
- [x] 06-04-PLAN.md - cmd/plunger/process.go full wiring + Update CatalogLoadedMsg follow-up (MOD-01..MOD-06)
- [x] 06-05-PLAN.md - cmd/plunger/monitor.go subcommand (MOD-07)
- [x] 06-06-PLAN.md - cmd/plunger/download.go subcommand + openURL helper (MOD-08)
- [x] 06-07-PLAN.md - main.go registration + E2E smoke test (MOD-01..MOD-08)
**UI hint**: yes

### Phase 7: Documentation & CI/CD
**Goal**: A beautiful, branded README that serves as the project's front door — clear install/build steps, full CLI reference, animated demo, and a tone that matches the pinball-arcade aesthetic. GitHub Actions pipeline for build+test+release. Hooks and processes that keep documentation accurate as the codebase evolves.
**Depends on**: Phase 6
**Requirements**: DOC-01, DOC-02, DOC-03, DOC-04, DOC-05, CI-01, CI-02, CI-03, CI-04
**Success Criteria** (what must be TRUE):
  1. README renders beautifully on GitHub: hero section with logo/badge strip, concise pitch, install block, CLI reference table, and aesthetic consistent with the Williams/Bally pinball theme
  2. `go build` / `make build` / `make snapshot` are documented with correct flags and produce a working binary
  3. All three subcommands (`process`, `monitor`, `download`) and their flags are documented with examples
  4. GitHub Actions CI runs `go test ./...` and `go build ./...` on every push/PR; release workflow produces tagged binaries via goreleaser
  5. A doc-freshness mechanism (hook or CI check) flags when CLI flags/subcommands change without a corresponding README update
**Plans** (4 plans):
- [x] 07-01-PLAN.md — README.md: hero, badges, pitch, install, quickstart, CLI ref, build docs (DOC-01, DOC-02, DOC-03)
- [x] 07-02-PLAN.md — Makefile fixes + .goreleaser.yml + .golangci.yml (DOC-02, CI-02, CI-03)
- [x] 07-03-PLAN.md — .github/workflows/ci.yml + release.yml + check-docs.sh (CI-01, CI-02, CI-03, CI-04, DOC-04, DOC-05)
- [ ] 07-04-PLAN.md — CONTRIBUTING.md + integration verification (DOC-04)

---

## Progress Table

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. TUI Skeleton | 7/7 | Complete | 2026-05-09 |
| 2. Classification & Archive Handling | 3/3 | Complete | 2026-05-09 |
| 3. Catalog & Matching Engine | 7/7 | Complete | 2026-05-10 |
| 4. Plan Builder | 6/6 | Complete   | 2026-05-14 |
| 5. Executor & Database | 4/4 | Complete | 2026-05-14 |
| 6. Full Binary | 7/7 | Complete | 2026-05-16 |
| 7. Documentation & CI/CD | 3/4 | In progress | — |

---

## Coverage

| Requirement | Phase |
|---|---|
| TUI-01 | Phase 1 |
| TUI-02 | Phase 1 |
| TUI-03 | Phase 1 |
| TUI-04 | Phase 1 |
| TUI-05 | Phase 1 |
| TUI-06 | Phase 1 |
| TUI-07 | Phase 1 |
| TUI-08 | Phase 1 |
| TUI-09 | Phase 1 |
| TUI-10 | Phase 1 |
| CLF-01 | Phase 2 |
| CLF-02 | Phase 2 |
| CLF-03 | Phase 2 |
| CLF-04 | Phase 2 |
| CLF-05 | Phase 2 |
| CLF-06 | Phase 2 |
| CLF-07 | Phase 2 |
| CLF-08 | Phase 2 |
| CLF-09 | Phase 2 |
| CLF-10 | Phase 2 |
| CLF-11 | Phase 2 |
| ARC-01 | Phase 2 |
| ARC-02 | Phase 2 |
| ARC-03 | Phase 2 |
| ARC-04 | Phase 2 |
| ARC-05 | Phase 2 |
| ARC-06 | Phase 2 |
| ARC-07 | Phase 2 |
| CAT-01 | Phase 3 |
| CAT-02 | Phase 3 |
| CAT-03 | Phase 3 |
| CAT-04 | Phase 3 |
| CAT-05 | Phase 3 |
| CAT-06 | Phase 3 |
| CAT-07 | Phase 3 |
| CAT-08 | Phase 3 |
| CAT-09 | Phase 3 |
| PLN-01 | Phase 4 |
| PLN-02 | Phase 4 |
| PLN-03 | Phase 4 |
| PLN-04 | Phase 4 |
| PLN-05 | Phase 4 |
| PLN-06 | Phase 4 |
| PLN-07 | Phase 4 |
| PLN-08 | Phase 4 |
| PLN-09 | Phase 4 |
| EXE-01 | Phase 5 |
| EXE-02 | Phase 5 |
| EXE-03 | Phase 5 |
| EXE-04 | Phase 5 |
| EXE-05 | Phase 5 |
| EXE-06 | Phase 5 |
| EXE-07 | Phase 5 |
| EXE-08 | Phase 5 |
| EXE-09 | Phase 5 |
| MOD-01 | Phase 6 |
| MOD-02 | Phase 6 |
| MOD-03 | Phase 6 |
| MOD-04 | Phase 6 |
| MOD-05 | Phase 6 |
| MOD-06 | Phase 6 |
| MOD-07 | Phase 6 |
| MOD-08 | Phase 6 |

**Total v1 requirements:** 63
**Mapped:** 63
**Unmapped:** 0

---

*Created: 2026-05-09*
