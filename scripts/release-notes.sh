#!/usr/bin/env bash
# Prints the "## <version>" section of CHANGELOG.md without its heading, as
# the notes of the GitHub release of that version, or fails when it has none.
# Usage: scripts/release-notes.sh <version>
set -euo pipefail

if [ $# -ne 1 ]; then
    sed -n '2,4s/^# \{0,1\}//p' "$0" >&2
    exit 1
fi
NOTES="$(awk -v heading="## $1" '
    $0 == heading { found = 1; next }
    found && /^## / { exit }
    found && /./ { printf "%s", blanks; blanks = ""; started = 1; print; next }
    found && started { blanks = blanks "\n" }
' "$(dirname "$0")/../CHANGELOG.md")"
if [ -z "$NOTES" ]; then
    echo "CHANGELOG.md has no notes under \"## $1\"." >&2
    exit 1
fi
printf '%s\n' "$NOTES"
