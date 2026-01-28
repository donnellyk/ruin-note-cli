#!/usr/bin/env bash
# scripts/test-vault.sh - Generate and manage test vaults for development

set -euo pipefail

DEFAULT_PATH="/tmp/ruin-test-vault"

usage() {
    cat <<EOF
Usage: $(basename "$0") <command> [path]

Commands:
  create [path]  Create a test vault with sample notes
  clean [path]   Remove a test vault (requires .ruin/ directory)
  reset [path]   Clean then create (fresh start)

Default path: $DEFAULT_PATH

Examples:
  $(basename "$0") create
  $(basename "$0") create ~/my-test-vault
  $(basename "$0") clean
  $(basename "$0") reset
EOF
    exit 1
}

create_note() {
    local vault_path="$1"
    local filename="$2"
    local uuid="$3"
    local created="$4"
    local tags="$5"
    local content="$6"

    cat > "$vault_path/$filename" <<EOF
---
uuid: $uuid
created: $created
updated: $created
tags: [$tags]
---

$content
EOF
}

create_vault() {
    local vault_path="$1"

    if [[ -d "$vault_path" ]]; then
        echo "Error: Directory already exists: $vault_path"
        echo "Use 'reset' to recreate or 'clean' first."
        exit 1
    fi

    echo "Creating test vault at: $vault_path"

    # Create vault and .ruin directory
    mkdir -p "$vault_path/.ruin"

    # Create empty metadata files
    cat > "$vault_path/.ruin/tags.yml" <<EOF
# Tag index for test vault
tags: []
EOF

    cat > "$vault_path/.ruin/queries.yml" <<EOF
# Saved queries for test vault
queries: []
EOF

    # Sample note 1: Daily Note
    create_note "$vault_path" "Daily-Note.md" "test-uuid-1" \
        "2025-01-28T09:00:00-08:00" \
        '"#daily", "#work"' \
        "# Daily Note

Started the day with some planning. #daily

## Tasks
- Review PRs
- Update documentation #work

End of day summary goes here."

    # Sample note 2: Project Ideas
    create_note "$vault_path" "Project-Ideas.md" "test-uuid-2" \
        "2025-01-27T14:30:00-08:00" \
        '"#project", "#idea"' \
        "# Project Ideas

## CLI Improvements #project

Some thoughts on improving the CLI experience.

## Future Features #idea

- Better search
- Tag autocomplete
- Graph visualization"

    # Sample note 3: Meeting Notes (with spaced tag)
    create_note "$vault_path" "Meeting-Notes.md" "test-uuid-3" \
        "2025-01-26T10:00:00-08:00" \
        '"#work", "#meeting notes#"' \
        "# Meeting Notes

#meeting notes# for the weekly sync.

## Attendees
- Alice
- Bob

## Discussion #work

Talked about upcoming deadlines and resource allocation."

    # Sample note 4: Personal Log
    create_note "$vault_path" "Personal-Log.md" "test-uuid-4" \
        "2025-01-25T20:15:00-08:00" \
        '"#daily", "#personal"' \
        "# Personal Log

#daily reflection on the week. #personal

## Gratitude
- Good progress on projects
- Helpful team

## Goals for Next Week
- Exercise more
- Read a book"

    # Sample note 5: Quick Thought (timestamp-named, minimal)
    local timestamp_name="2025-01-28T15-45-00.md"
    create_note "$vault_path" "$timestamp_name" "test-uuid-5" \
        "2025-01-28T15:45:00-08:00" \
        '"#idea"' \
        "Quick thought: what if we added fuzzy search? #idea"

    echo "Created test vault with 5 sample notes:"
    echo "  - Daily-Note.md"
    echo "  - Project-Ideas.md"
    echo "  - Meeting-Notes.md"
    echo "  - Personal-Log.md"
    echo "  - $timestamp_name"
    echo ""
    echo "Test with: ./ruin --vault $vault_path log"
}

clean_vault() {
    local vault_path="$1"

    if [[ ! -d "$vault_path" ]]; then
        echo "Directory does not exist: $vault_path"
        return 0
    fi

    # Safety check: only delete if .ruin/ directory exists
    if [[ ! -d "$vault_path/.ruin" ]]; then
        echo "Error: Not a ruin vault (no .ruin/ directory): $vault_path"
        echo "Refusing to delete for safety."
        exit 1
    fi

    echo "Removing test vault: $vault_path"
    rm -rf "$vault_path"
    echo "Done."
}

# Main
case "${1:-}" in
    create)
        create_vault "${2:-$DEFAULT_PATH}"
        ;;
    clean)
        clean_vault "${2:-$DEFAULT_PATH}"
        ;;
    reset)
        clean_vault "${2:-$DEFAULT_PATH}" || true
        create_vault "${2:-$DEFAULT_PATH}"
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        usage
        ;;
esac
