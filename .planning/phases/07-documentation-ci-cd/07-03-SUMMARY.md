---
phase: 07-documentation-ci-cd
plan: 03
subsystem: infra
tags: [github-actions, goreleaser, golangci-lint, ci-cd, shell-script]

requires:
  - phase: 07-02
    provides: .goreleaser.yml and .golangci.yml needed for CI/release config references

provides:
  - GitHub Actions CI pipeline (lint + test + build + doc-freshness) on push/PR to master
  - GitHub Actions release pipeline (goreleaser-action@v7 on v* tags)
  - Doc-freshness shell script asserting all cobra flags appear in README.md

affects: [readme, goreleaser, golangci-lint, release]

tech-stack:
  added:
    - goreleaser/goreleaser-action@v7
    - golangci/golangci-lint-action@v6
    - actions/setup-go@v5 (built-in module cache)
    - actions/checkout@v4
  patterns:
    - Four parallel CI jobs (no needs: dependencies) for maximum throughput
    - GNU grep -oP with BASH_SOURCE self-location for portable doc-freshness script
    - git update-index --chmod=+x to set executable bit on Windows without chmod

key-files:
  created:
    - .github/workflows/ci.yml
    - .github/workflows/release.yml
    - .github/scripts/check-docs.sh
  modified: []

key-decisions:
  - "goreleaser-action@v7 with version: ~> v2 and args: release --clean (not --rm-dist, removed in v2)"
  - "fetch-depth: 0 mandatory in release.yml checkout (goreleaser changelog needs full git history)"
  - "permissions: contents: write at workflow level in release.yml (goreleaser creates GitHub Release)"
  - "doc-freshness CI job pinned to ubuntu-latest (GNU grep -oP not available on macOS runners)"
  - "check-docs.sh is self-locating via BASH_SOURCE (can be invoked from any CWD)"
  - "setup-go@v5 built-in cache replaces manual actions/cache step"

patterns-established:
  - "Pattern: parallel CI jobs with no needs: for maximum throughput"
  - "Pattern: doc-freshness script as standalone bash file called by CI step"

requirements-completed: [CI-01, CI-02, CI-03, CI-04, DOC-04, DOC-05]

duration: 12min
completed: 2026-05-16
---

# Phase 07 Plan 03: CI/CD Workflows and Doc-Freshness Summary

**GitHub Actions CI (four parallel jobs) and release pipeline via goreleaser-action@v7, plus a GNU grep doc-freshness script asserting all cobra flags are documented in README.md**

## Performance

- **Duration:** 12 min
- **Started:** 2026-05-16T16:10:00Z
- **Completed:** 2026-05-16T16:22:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Created `.github/workflows/ci.yml` with four parallel jobs (lint/golangci-lint-action@v6, test/go test -race, build/CGO_ENABLED=0, doc-freshness/check-docs.sh) triggering on push and PR to master
- Created `.github/workflows/release.yml` with goreleaser-action@v7 triggering on v* tags, with fetch-depth: 0 and permissions: contents: write
- Created `.github/scripts/check-docs.sh` using GNU grep -oP to extract cobra flag names from cmd/plunger/ and assert each appears in README.md; exits non-zero with specific error listing missing flags

## Task Commits

1. **Task 1: Create ci.yml** - `3bde96f` (feat)
2. **Task 2: Create release.yml and check-docs.sh** - `4f284fb` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `.github/workflows/ci.yml` - Four parallel CI jobs: lint (golangci-lint-action@v6), test (go test ./... -v -race), build (CGO_ENABLED=0 go build ./...), doc-freshness (bash check-docs.sh)
- `.github/workflows/release.yml` - Release pipeline: goreleaser-action@v7 on v* tags with fetch-depth: 0 and permissions: contents: write
- `.github/scripts/check-docs.sh` - Self-locating bash script using GNU grep -oP to extract cobra flag registrations, asserts each flag name in README.md, exits 1 with error list on failure

## Decisions Made

- `goreleaser-action@v7` with `version: "~> v2"` and `args: release --clean` — the `--rm-dist` flag was removed in goreleaser v2; `--clean` is the correct replacement
- `fetch-depth: 0` is mandatory in release.yml because goreleaser reads full git history to generate the changelog; shallow clones break it
- `permissions: contents: write` at workflow level required for goreleaser to create GitHub Releases (without it goreleaser gets HTTP 403)
- doc-freshness job runs on `ubuntu-latest` only because `grep -oP` (Perl regex) requires GNU grep; macOS runners ship BSD grep which lacks `-P`
- `setup-go@v5` with `cache: true` replaces any manual `actions/cache` step — built-in caching since v5
- `git update-index --chmod=+x` used to set executable bit on Windows (no `chmod` available in PowerShell/Bash on this dev machine)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `git update-index --chmod=+x` required staging the file first (bare `--chmod=+x` without `--add` fails on untracked files). Resolved by running `git add` before `git update-index`.

## User Setup Required

None - no external service configuration required. GITHUB_TOKEN is the built-in automatic secret; no manual token creation needed.

## Next Phase Readiness

- CI/CD infrastructure complete for Phase 7
- All three files committed and ready to activate on next push to GitHub
- Release pipeline will trigger on any `v*` tag push
- Doc-freshness check will enforce README stays in sync with cobra flag changes

---
*Phase: 07-documentation-ci-cd*
*Completed: 2026-05-16*
