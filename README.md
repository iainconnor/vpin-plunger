<pre>
        |
        |
       ( )
      /   \
     |     |
      \   /
       ---
       ###

  P L U N G E R
</pre>

**An opinionated tool to build and manage your virtual pinball collection.**

---

[![Build](https://github.com/iainconnor/vpin-plunger/actions/workflows/ci.yml/badge.svg)](https://github.com/iainconnor/vpin-plunger/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/iainconnor/vpin-plunger)](LICENSE)
[![Release](https://img.shields.io/github/v/release/iainconnor/vpin-plunger)](https://github.com/iainconnor/vpin-plunger/releases/latest)

---

## Philosophy

Most virtual pinball tools are file renamers wearing a suit. Plunger is something different.

The idea is simple: your collection should be *curated*, not just *accumulated*. Plunger is built around the "all killer, no filler" philosophy — you pick a target collection (say, every Williams machine from the golden era, every Stern title you grew up pouring quarters into), and Plunger helps you build toward it systematically.

In practice, Plunger does the heavy lifting that cabinet operators dread:

- Scans a flat `downloads/` folder containing whatever you pulled from VPinballX.com or VirtualPinballSpreadsheet.com
- Classifies every file by asset type: table, backglass, ROM, NVRAM, POV, DMD pack, FlexDMD asset, audio pack, colorization pack, music track, PuP pack
- Fuzzy-matches each asset against the community catalog to find the canonical `{Name} ({Manufacturer}, {Year})` identifier
- Builds a plan: what moves where, what gets renamed, what goes to the review queue
- **Shows you the plan before doing anything** — dry-run is the default mindset
- On your confirmation, executes the moves, renames files to canonical format, and registers new tables in PinUP Popper's database

What makes Plunger different is that last step before execution. The planner always shows you the plan. You pull the trigger.

Coming in a future release: collection-goal tracking — define your target collection and Plunger measures your progress and surfaces what's missing.

---

## How It Works

1. **Download** tables, backglasses, ROMs, and other assets from [VPinballX.com](https://vpinballx.com) or [VirtualPinballSpreadsheet.com](https://virtualpinballspreadsheet.com) into a flat `downloads/` folder. No subfolders needed — Plunger handles the sorting.

2. **Run** `vpin process --dir C:\Users\Me\Downloads\VPinball`

3. **Plunger scans** the directory and classifies every file. Archives are inspected without fully extracting them. ROM zips are identified and kept intact. PuP packs are matched by directory name.

4. **Fuzzy matching** runs against the community catalog. High-confidence matches (92%+) are assigned automatically. Ambiguous matches (72–91%) prompt you interactively. Anything below 72% goes to the review queue — never silently discarded.

5. **The plan appears**: a dry-run display showing exactly what would move, what would be renamed, what would go to review. No files have moved yet.

6. **Confirm**, and Plunger executes — files land in the right VPX paths, Popper's database is updated, and a `REVIEW.md` file captures anything that needed a human eye.

---

## Install

### Download the Windows binary (recommended)

Download `vpin.exe` from the latest release:

```
https://github.com/iainconnor/vpin-plunger/releases/latest
```

No installer. Drop `vpin.exe` somewhere on your PATH (e.g., `C:\vPinball\Tools\`) and you're done. No runtime dependencies. No .NET. No Python. Just a single native binary.

### Build from source

See the [Build from source](#build-from-source) section below for full instructions.

Quick version:

```powershell
git clone https://github.com/iainconnor/vpin-plunger.git
cd vpin-plunger
make build
```

---

## Quick Start

```powershell
vpin process --dir "C:\Users\Me\Downloads\VPinball"
```

Plunger will scan the directory, match each file against the community catalog, show you the plan, and wait for your confirmation before moving anything.

Want to see what it would do without touching a single file?

```powershell
vpin process --dir "C:\Users\Me\Downloads\VPinball" --dry-run
```

---

## Subcommand Reference

### `vpin process`

Scans a downloads directory, classifies every asset, fuzzy-matches against the community catalog, builds a move plan, and (after confirmation) executes it — renaming files to canonical format, moving them to the correct VPX install paths, and registering new tables in PinUP Popper.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--db` | string | `C:\vPinball\PinUPSystem\PUPDatabase.db` | Path to PUPDatabase.db |
| `--dir` | string | *(required)* | Path to downloads/ directory to scan |
| `--auto` | bool | false | Non-interactive: send unmatched files to review automatically |
| `--dry-run` | bool | false | Build and display the plan without executing |
| `--rehearsal` | bool | false | Execute against a sandboxed `rehearsal/` subdirectory |
| `--vpx-dir` | string | `C:\vPinball\visualpinball\Tables` | Override VPX install directory |
| `--backglass-dir` | string | `C:\vPinball\visualpinball\Tables` | Override backglass directory |
| `--rom-dir` | string | `C:\vPinball\visualpinball\VPinMAME\roms` | Override ROM directory |
| `--nvram-dir` | string | `C:\vPinball\visualpinball\VPinMAME\nvram` | Override NVRAM saves directory |
| `--pov-dir` | string | `C:\vPinball\visualpinball\Tables` | Override POV (point-of-view) directory |
| `--dmd-dir` | string | `C:\vPinball\visualpinball\UltraDMD` | Override UltraDMD directory |
| `--flexdmd-dir` | string | `C:\vPinball\visualpinball\FlexDMD` | Override FlexDMD directory |
| `--audio-dir` | string | `C:\vPinball\visualpinball\VPinMAME\altsound` | Override Altsound directory |
| `--altcolor-dir` | string | `C:\vPinball\visualpinball\VPinMAME\altcolor` | Override Altcolor/colorization directory |
| `--music-dir` | string | `C:\vPinball\visualpinball\Music` | Override music directory |
| `--pup-dir` | string | `C:\vPinball\PinUPSystem\POPMedia\PuPPacks` | Override PuP packs directory |

**Examples:**

```powershell
# Dry run first — see what Plunger would do, nothing moves
vpin process --dir "C:\Users\Me\Downloads\VPinball" --dry-run

# Non-interactive full run (great for scripting or overnight runs)
vpin process --dir "C:\Users\Me\Downloads\VPinball" --auto

# Safe rehearsal — all moves happen inside a rehearsal/ sandbox, nothing goes to real dirs
vpin process --dir "C:\Users\Me\Downloads\VPinball" --rehearsal

# Override specific directories if your layout differs from Baller Installer standard
vpin process --dir "C:\Downloads" --vpx-dir "D:\Tables" --rom-dir "D:\MAME\roms"
```

---

### `vpin monitor`

Compare your installed PUPDatabase against the community catalog. Surfaces tables not yet installed, unknown tables in your collection, and GameName mismatches between what Popper thinks the table is called and what the catalog says the canonical name is.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--db` | string | `C:\vPinball\PinUPSystem\PUPDatabase.db` | Path to PUPDatabase.db |
| `--dir` | string | *(optional)* | Path to downloads/ directory (used to locate catalog cache) |

**Example:**

```powershell
vpin monitor --db "C:\vPinball\PinUPSystem\PUPDatabase.db"
```

Monitor is read-only — it never modifies your database or files. Think of it as a health check for your collection's metadata.

---

### `vpin download`

Find uninstalled catalog entries, grouped by manufacturer and decade, and open their VPinballX / VirtualPinballSpreadsheet URLs in the browser. Great for discovering what's available for a particular era of machines.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--db` | string | `C:\vPinball\PinUPSystem\PUPDatabase.db` | Path to PUPDatabase.db |
| `--dir` | string | *(optional)* | Path to downloads/ directory |
| `--dry-run` | bool | false | Print URLs instead of opening them |

**Examples:**

```powershell
# See what URLs would open — dry-run prints them without opening anything
vpin download --dry-run

# Open download URLs in the browser (grouped by manufacturer and decade)
vpin download
```

---

## Configuration

Plunger follows the [Baller Installer](https://baller-installer.com) standard path layout out of the box. If your cabinet uses a different layout, every path is overridable via flag.

### Default Path Layout

| Asset Type | Default Path | Override Flag |
|------------|-------------|---------------|
| PUPDatabase | `C:\vPinball\PinUPSystem\PUPDatabase.db` | `--db` |
| VPX tables | `C:\vPinball\visualpinball\Tables` | `--vpx-dir` |
| Backglasses | `C:\vPinball\visualpinball\Tables` | `--backglass-dir` |
| ROM archives | `C:\vPinball\visualpinball\VPinMAME\roms` | `--rom-dir` |
| NVRAM saves | `C:\vPinball\visualpinball\VPinMAME\nvram` | `--nvram-dir` |
| POV files | `C:\vPinball\visualpinball\Tables` | `--pov-dir` |
| UltraDMD packs | `C:\vPinball\visualpinball\UltraDMD` | `--dmd-dir` |
| FlexDMD assets | `C:\vPinball\visualpinball\FlexDMD` | `--flexdmd-dir` |
| Altsound packs | `C:\vPinball\visualpinball\VPinMAME\altsound` | `--audio-dir` |
| Colorization | `C:\vPinball\visualpinball\VPinMAME\altcolor` | `--altcolor-dir` |
| Music tracks | `C:\vPinball\visualpinball\Music` | `--music-dir` |
| PuP packs | `C:\vPinball\PinUPSystem\POPMedia\PuPPacks` | `--pup-dir` |

### Confidence Thresholds

Plunger uses fuzzy matching against the community catalog. The thresholds:

| Confidence | Action |
|------------|--------|
| 92% and above | Auto-assign — file is matched and moved without prompting |
| 72%–91% | Interactive prompt — Plunger shows you the candidates and asks |
| Below 72% | Sent to review queue — captured in `REVIEW.md` for manual resolution |

### Catalog Staleness

The community catalog is cached locally after the first download. By default, Plunger considers the cache stale after 7 days and will offer to refresh it. If you're processing a large batch and don't want the refresh prompt, use `--auto` — it accepts the cached catalog as-is.

---

## Build from Source

### Prerequisites

- **Go** (latest stable, download from [go.dev](https://go.dev)) — `CGO_ENABLED=0` always; no C toolchain needed
- **goreleaser** (optional, for multi-platform snapshot builds) — `winget install goreleaser.goreleaser`
- **golangci-lint** (optional, for local linting) — `winget install golangci.golangci-lint`

### Steps

```powershell
# Clone the repository
git clone https://github.com/iainconnor/vpin-plunger.git
cd vpin-plunger

# Build for the current platform
make build
# Output: dist/vpin.exe (Windows) or dist/vpin (Linux)

# Run the test suite
make test

# Run the linter
make lint

# Multi-platform snapshot build (requires goreleaser)
make snapshot
# Output: dist/vpin_{version}_{os}_{arch}.{zip,tar.gz}
```

Alternatively, build directly with Go:

```powershell
# Windows (PowerShell)
$env:CGO_ENABLED = "0"
& "C:\Program Files\Go\bin\go.exe" build -trimpath -o dist\vpin.exe .\cmd\plunger

# Linux / macOS
CGO_ENABLED=0 go build -trimpath -o dist/vpin ./cmd/plunger
```

The `CGO_ENABLED=0` flag is always required. Plunger uses no C dependencies — the binary is pure Go and runs anywhere the Go runtime does.

### Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build for current platform into `dist/` |
| `make test` | Run `go test ./... -v -race` |
| `make lint` | Run golangci-lint |
| `make snapshot` | goreleaser snapshot (all platforms, no tag required) |
| `make release` | goreleaser release (requires GITHUB_TOKEN and a version tag) |
| `make clean` | Remove `dist/` |

---

## Coming Soon

**Collection-goal tracking:** Define your target collection — say, every Williams machine from 1990–1999, or every Stern title from the Spike era — and Plunger measures how far along you are and identifies gaps. The catalog data is already here; the goal-tracking layer is coming in a future release.

This is what separates a collection from a pile of `.vpx` files.

---

## Contributing

Pull requests welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions, coding conventions, and the contribution workflow.

The short version: fork, branch, write tests, run `make test` and `make lint`, open a PR against `master`. Please don't skip the tests — every phase ships with `go test` passing, and we'd like to keep it that way.
