#!/bin/bash
# Extract the latest version from changelog.md
# Usage: ./scripts/get-version.sh
# Output: version string, with "-unreleased-HASH" appended if UNRELEASED is in changelog

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CHANGELOG="$PROJECT_ROOT/changelog.md"

if [ ! -f "$CHANGELOG" ]; then
  echo "error: changelog.md not found" >&2
  exit 1
fi

# Extract the first version line (e.g., "## [0.5.0] - UNRELEASED")
VERSION_LINE=$(grep -m1 '## \[\([0-9.]*\)\]' "$CHANGELOG")

if [ -z "$VERSION_LINE" ]; then
  echo "error: no version found in changelog.md" >&2
  exit 1
fi

# Extract version number (e.g., "0.5.0" from "## [0.5.0] - UNRELEASED")
VERSION=$(echo "$VERSION_LINE" | sed -E 's/## \[([0-9.]+)\].*/\1/')

# Check if the version line contains "UNRELEASED"
if echo "$VERSION_LINE" | grep -q "UNRELEASED"; then
  # Get git describe output for full context (commit count, hash, dirty status)
  GIT_DESC=$(cd "$PROJECT_ROOT" && git describe --tags --always --dirty 2>/dev/null || echo "dev")
  VERSION="${VERSION}-unreleased-${GIT_DESC}"
fi

echo "$VERSION"
