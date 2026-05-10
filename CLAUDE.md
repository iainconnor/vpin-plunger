# vpin-plunger
https://github.com/iainconnor/vpin-plunger

Manages and automates a curated collection of virtual pinball tables
and related assets. Headless TUI. Targets Windows (PowerShell /
Windows Terminal) and Linux. Single native binary, no runtime deps.

## Stack
- Language:   Go (latest stable)
- TUI:        bubbletea v2 ? charm.land/bubbletea/v2
- Components: bubbles ? github.com/charmbracelet/bubbles
- Styling:    lipgloss ? github.com/charmbracelet/lipgloss
- Archives:   mholt/archives ? zip, rar, 7z, tar, gz, bz2
- Build:      make build / make snapshot ? CGO_ENABLED=0 always
- Deps:       make tidy ? flag any new dependency before adding

Dependencies are anchored in skeleton files that import and use
each package. Do not work around these imports. Do not reimplement
anything a required library already provides.

## Layout
cmd/plunger/         ? main.go, wiring only
internal/
  app/               ? bubbletea Model, Update, View
  formats/           ? all format and archive logic
  ui/
    theme/           ? all colors and styles (named constants only)
    components/      ? TUI components wrapping bubbles primitives
  config/

## Architecture
Strict bubbletea Elm. No exceptions.
- Model: all state
- Update(msg): only place state changes
- View(): pure, no I/O, no side effects
- Concurrency: tea.Cmd only, no raw goroutines

## UI ? 90s Pinball Aesthetic
Williams/Bally machines in a dark arcade. Dot matrix amber on cabinet
black. Saturated neon inserts. "I didn't know a CLI could do that."

Palette (all in internal/ui/theme/colors.go ? never hardcoded elsewhere):
  ColorBackground  #0A0A0F
  ColorInactive    #1A1A2E
  ColorStatusBar   #12122A
  ColorDMD         #FF8C00
  ColorAccent      #00BFFF
  ColorSuccess     #39FF14
  ColorWarning     #FF006E
  ColorMuted       #6B7FA3

Each asset type gets a distinct named insert color in theme/colors.go,
assigned during milestone 1. Vivid and saturated. No two types share
a color.

Status bar: DMD readout style. Amber. Machine-state language.
Progress: playfield lane fill style.
Spinners: bubbles Dot style.
Asset labels: colored rounded pill badges.

## Contracts
- Pinned status bar always visible
- Determinate progress (%) or indeterminate (spinner) as appropriate
- Graceful terminal resize always

## Format & Archive Rules
- All format logic in internal/formats/ only
- Archive traversal via Walk() in formats/format.go (mholt/archives)
- formats/ has zero dependency on ui/
- New format = changes inside internal/formats/ only

## Do Not
- Raw goroutines ? use tea.Cmd
- Logic in View()
- Hardcoded colors or styles outside theme/
- Coupling between formats/ and ui/
- CGO
- Undisclosed new dependencies
- Logic in main.go

## Windows Environment
- Go binary: `C:\Program Files\Go\bin\go.exe` (not in Bash PATH — use full path or PowerShell)
- All `go build`, `go test`, `go mod tidy` commands must use the full path when invoked via Bash tool
- PowerShell has Go available via full path: `& "C:\Program Files\Go\bin\go.exe" ...`

---

## GSD Workflow

Project initialized with GSD. Planning docs live in `.planning/` (git-ignored, local only).

**Current roadmap:** 6 phases — TUI Skeleton ? Classification ? Catalog ? Plan Builder ? Executor ? Full Binary

**Active workflow commands:**
- `/gsd-plan-phase N` — plan the next phase before executing
- `/gsd-execute-phase N` — execute a planned phase
- `/gsd-progress` — check current phase status
- `/gsd-discuss-phase N` — discuss approach before planning

**Per-phase testing requirement:** Every phase must ship with `go test` passing for that phase's packages. No test debt deferred to Phase 6.

**Execution mode:** YOLO (auto-approve). Plans run in parallel where independent.
