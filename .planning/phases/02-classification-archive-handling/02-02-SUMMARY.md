---
phase: 02-classification-archive-handling
plan: "02"
subsystem: formats
tags: [archive-handlers, zip, sevenzip, rar, go-modules]
dependency_graph:
  requires: [02-01]
  provides: [ZIPHandler, SevenZipHandler, RARHandler, wrapRARError, testdata fixtures]
  affects: [02-03]
tech_stack:
  added: []
  patterns: [zip-slip protection, rardecode error wrapping, TestMain fixture generation]
key_files:
  created:
    - internal/formats/handlers.go
    - internal/formats/handlers_test.go
    - internal/formats/testdata/.gitkeep
  modified:
    - go.mod
    - go.sum
decisions:
  - bodgit/sevenzip and nwaples/rardecode/v2 promoted from indirect to direct deps via go mod tidy
  - ErrUnknownVersion confirmed present in nwaples/rardecode/v2@v2.2.0 (reader.go:37); used as documented in plan
  - RAR Peek test skips gracefully when multi-volume part02 is absent; error-wrapping test always runs
  - 7z fixture created via 7z.exe CLI at C:\Program Files\7-Zip\7z.exe (version 24.07 present)
metrics:
  duration: "~15 minutes"
  completed: "2026-05-09"
  tasks_completed: 2
  files_created: 3
  files_modified: 2
---

# Phase 02 Plan 02: Archive Handler Implementations Summary

ZIPHandler/SevenZipHandler/RARHandler implementing the Handler interface with identical zip-slip protection and consistent error wrapping; ARC-01 through ARC-06 test coverage.

## Tasks Completed

| # | Task | Commit | Status |
|---|------|--------|--------|
| 1 | handlers.go: ZIPHandler, SevenZipHandler, RARHandler + wrapRARError; go mod tidy | f4f67e7 | DONE |
| 2 | testdata fixtures + handlers_test.go (ARC-01..ARC-06) | dd90d10 | DONE |

## Exported Types and Methods

### ZIPHandler (archive/zip stdlib — D-08)
- `func (ZIPHandler) Name() string` → `"ZIP"`
- `func (ZIPHandler) Detect(_ context.Context, p string, _ fs.File) bool` — ext `.zip` (case-insensitive)
- `func (ZIPHandler) Peek(p string) ([]string, error)` — flat member name list, backslash normalised to `/`
- `func (ZIPHandler) Extract(src, dest string) error` — all members extracted with zip-slip protection

### SevenZipHandler (bodgit/sevenzip v1.6.1 — D-09)
- `func (SevenZipHandler) Name() string` → `"7z"`
- `func (SevenZipHandler) Detect(_ context.Context, p string, _ fs.File) bool` — ext `.7z` (case-insensitive)
- `func (SevenZipHandler) Peek(p string) ([]string, error)` — flat member name list via sevenzip.OpenReader
- `func (SevenZipHandler) Extract(src, dest string) error` — dir-entry guard applied (Pitfall 2), zip-slip protection

### RARHandler (nwaples/rardecode/v2 v2.2.0 — D-10)
- `func (RARHandler) Name() string` → `"RAR"`
- `func (RARHandler) Detect(_ context.Context, p string, _ fs.File) bool` — ext `.rar` (case-insensitive)
- `func (RARHandler) Peek(p string) ([]string, error)` — via rardecode.List; errors wrapped by wrapRARError
- `func (RARHandler) Extract(src, dest string) error` — OpenReader + Next() sequential loop; zip-slip protection

### wrapRARError
- Translates `rardecode.ErrSolidOpen`, `rardecode.ErrArchivedFileEncrypted`, `rardecode.ErrUnknownVersion` to operator-readable messages
- Default case: `fmt.Errorf("rar %s %s: %w", op, p, err)`

## go.mod Promotion Confirmation

Both libraries promoted from `// indirect` to direct `require` block after `go mod tidy`:
```
github.com/bodgit/sevenzip v1.6.1
github.com/nwaples/rardecode/v2 v2.2.0
```

## Sentinel Error Name Verification

- `rardecode.ErrSolidOpen` — confirmed at `reader.go:36` in v2.2.0
- `rardecode.ErrArchivedFileEncrypted` — confirmed at `archive.go:26` in v2.2.0
- `rardecode.ErrUnknownVersion` — confirmed at `reader.go:37` in v2.2.0 (matches plan; no deviation)

## Test Results

| Test | Result | Notes |
|------|--------|-------|
| TestZIPHandler_Peek | PASS | dist.zip member list includes game.vpx and altsound/altsound.csv |
| TestZIPHandler_Peek_FeedsIsROMZip_ARC01_CLF03 | PASS | rom.zip → IsROMZip true; dist.zip → IsROMZip false |
| TestZIPHandler_Extract_ARC02 | PASS | game.vpx and altsound/altsound.csv extracted correctly |
| TestZIPHandler_Extract_RejectsZipSlip | PASS | ../escape.txt rejected; error contains "path escape" |
| TestSevenZipHandler_Peek_ARC03 | PASS | 7z.exe at C:\Program Files\7-Zip\7z.exe present; dist.7z built; game.vpx in member list |
| TestSevenZipHandler_Extract_ARC04 | PASS | game.vpx found in extracted temp dir |
| TestRARHandler_Peek_ARC05 | SKIP | test.part01.rar is a multi-volume set needing part02; rardecode.List returns error |
| TestRARHandler_Extract_ErrorWrapping_ARC06 | PASS | non-existent path returns error containing "rar extract" |

## Deviations from Plan

None — plan executed exactly as written. All three sentinel error names in rardecode v2.2.0 matched the plan's documented names.

## Known Stubs

None. All handler methods are fully implemented and tested.

## Threat Flags

None. No new network endpoints, auth paths, or schema changes introduced. Zip-slip protection (T-02-04) implemented in all three Extract methods. wrapRARError handles T-02-07.

## Next Steps

- Plan 03 (walk.go) delivers ARC-07: Walk() integration that composes the three handlers via mholt/archives for unified archive traversal.

## Self-Check: PASSED

- `internal/formats/handlers.go` exists: FOUND
- `internal/formats/handlers_test.go` exists: FOUND
- `internal/formats/testdata/.gitkeep` exists: FOUND
- Commit f4f67e7 exists: FOUND
- Commit dd90d10 exists: FOUND
- `go test ./internal/formats/...` passes (1 skip for multi-volume RAR, 0 failures): CONFIRMED
