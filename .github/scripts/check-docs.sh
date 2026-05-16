#!/usr/bin/env bash
# .github/scripts/check-docs.sh
# Greps cobra flag registrations from cmd/plunger/ and asserts each flag name
# appears in README.md. Exits non-zero and lists undocumented flags.
# Requires GNU grep (ubuntu-latest). Do not run on macOS runners.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CMD_DIR="$REPO_ROOT/cmd/plunger"
README="$REPO_ROOT/README.md"

if [ ! -f "$README" ]; then
    echo "ERROR: README.md not found at $README"
    exit 1
fi

# Extract flag names: match .Flags().(String|Bool|Int)Var(ptr, "flagname", ...
# Using grep -oP (Perl regex, GNU grep only) to capture the flag name.
flags=$(grep -rhoP '\.Flags\(\)\.(String|Bool|Int)Var\([^,]+,\s*"\K[^"]+' "$CMD_DIR" | sort -u)

if [ -z "$flags" ]; then
    echo "ERROR: No flags found in $CMD_DIR — check grep pattern"
    exit 1
fi

missing=()
while IFS= read -r flag; do
    if ! grep -qF -- "--${flag}" "$README" && ! grep -qF -- "\`${flag}\`" "$README"; then
        missing+=("--${flag}")
    fi
done <<< "$flags"

flag_count=$(echo "$flags" | wc -l | tr -d ' ')

if [ ${#missing[@]} -gt 0 ]; then
    echo "ERROR: The following CLI flags are not documented in README.md:"
    printf '  %s\n' "${missing[@]}"
    echo ""
    echo "Please update README.md to document these flags."
    exit 1
fi

echo "OK: All ${flag_count} CLI flags are documented in README.md."
