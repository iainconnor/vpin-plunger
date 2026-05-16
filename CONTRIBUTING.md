# Contributing to Plunger

Welcome, and thank you for wanting to contribute to Plunger. This is a hobby project built for virtual pinball cabinet operators by someone who is definitely spending too much time thinking about file naming conventions. All skill levels are welcome here — if you can operate a cabinet, you probably know more about the domain than most developers ever will. Be kind; we're all just trying to make the hobby a little more polished.

---

## Prerequisites

You need Go and `make`. Everything else is optional.

| Tool | Required | Install |
|------|----------|---------|
| **Go** (latest stable) | Yes | [go.dev](https://go.dev) — `go version` should say 1.21+ |
| **make** | Yes | Ships with Git for Windows; standard on Linux/macOS |
| **goreleaser** | Optional | For multi-platform snapshot builds |
| **golangci-lint** | Optional | For running the linter locally |

### Install optional tools

```powershell
# Windows (winget)
winget install goreleaser.goreleaser
winget install golangci.golangci-lint
```

```bash
# Linux/macOS (brew)
brew install goreleaser golangci-lint
```

---

## Getting Started

```bash
git clone https://github.com/iainconnor/vpin-plunger.git
cd vpin-plunger
make build        # build for current platform → dist/vpin (or dist/vpin.exe on Windows)
make test         # run all tests (go test ./... -v -race)
make lint         # run golangci-lint
```

The build is always `CGO_ENABLED=0` — no C toolchain required, no cgo, no surprises.

---

## Project Structure

```
cmd/plunger/         — main.go, wiring only (no logic here)
internal/
  app/               — bubbletea Model/Update/View (strict Elm architecture)
  formats/           — all format and archive logic (no UI dependency)
  ui/
    theme/           — colors.go only (never hardcode colors elsewhere)
    components/      — TUI components wrapping bubbles primitives
  config/            — CLI flag → Config translation layer
  catalog/           — Google Sheets catalog download and matching
  planner/           — two-pass scan and ProcessPlan tree
  executor/          — file moves, archive extraction, REVIEW.md
  db/                — PUPDatabase upsert and backup rotation
```

The `internal/formats/` package has zero dependency on `ui/` or `catalog/`. This is intentional — it means format classification and archive traversal can be unit tested without touching the TUI.

---

## Architecture Rules (the important ones)

These constraints exist to keep the codebase predictable and testable. Please follow them:

- **No raw goroutines** — all concurrency via `tea.Cmd`. This is strict bubbletea Elm; raw goroutines bypass the message bus and cause races.
- **No logic in `View()`** — `View()` is a pure render function. All state changes happen in `Update()`. If you're doing an `if` in `View()` that isn't purely about how to display something, move it to `Update()`.
- **No hardcoded colors** — every `lipgloss.Color` value must live in `internal/ui/theme/colors.go`. No exceptions. Named constants only.
- **`formats/` has zero UI dependency** — `internal/formats/` must never import from `ui/` or `catalog/`. This keeps classification logic independently testable.
- **`CGO_ENABLED=0` always** — Plunger has no C dependencies. This is a hard constraint; the CI pipeline enforces it.
- **No new dependencies without flagging** — run `make tidy` to verify the module graph, and raise any new dependency in your PR description. We want to be intentional about what we pull in.
- **`main.go` is wiring only** — `cmd/plunger/main.go` connects the bubbletea runtime to the app model. No domain logic goes there.

---

## Submitting a PR

1. Fork the repo and create a branch: `git checkout -b feature/my-improvement`
2. Make your changes and write tests for any new behavior
3. Ensure the tests pass: `make test`
4. Ensure the linter passes: `make lint`
5. Push your branch and open a PR against `master`
6. In the PR description, explain what you changed and why — context matters more than a diff

Please don't skip the tests. Every phase of this project shipped with `go test` passing, and we'd like to keep it that way.

---

## Adding a New Asset Type (Format Support)

All format logic lives in `internal/formats/`. To add support for a new asset type:

1. Add a new classifier rule in `internal/formats/classify.go`
2. Add a corresponding test case in `internal/formats/classify_test.go`
3. Add a named insert color for the new type in `internal/ui/theme/colors.go` (each asset type gets a distinct, vivid color — no two types share one)

The `formats/` package must never import from `ui/` or `catalog/`. Keep it that way.

---

## Doc Updates

If you add or rename CLI flags, update `README.md` — the CI doc-freshness check (`check-docs.sh`) will fail if any flag is undocumented. The check runs as part of the CI pipeline on every push and PR against master. It greps cobra flag registrations from `cmd/plunger/` and asserts each flag name appears in `README.md`.

To verify locally before pushing (requires Git Bash on Windows / any bash on Linux):

```bash
bash .github/scripts/check-docs.sh
```

Expected output: `OK: All N CLI flags are documented in README.md.`

---

## Make Targets Reference

| Target | Description |
|--------|-------------|
| `make build` | Build for current platform into `dist/` |
| `make test` | Run `go test ./... -v -race` |
| `make lint` | Run golangci-lint |
| `make snapshot` | goreleaser snapshot (all platforms, no tag required) |
| `make release` | goreleaser release (requires GITHUB_TOKEN and a version tag) |
| `make clean` | Remove `dist/` |
| `make tidy` | Run `go mod tidy` |
