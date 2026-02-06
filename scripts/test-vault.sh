#!/usr/bin/env bash
# scripts/test-vault.sh - Generate and manage test vaults for development

set -euo pipefail

DEFAULT_PATH="/tmp/ruin-test-vault"

usage() {
    cat <<EOF
Usage: $(basename "$0") <command> [path]

Commands:
  create [path]  Create a test vault with 100 sample notes
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

# pick - select a random item from a |-delimited string
# Usage: pick "apple|banana|cherry"
pick() {
    local pool="$1"
    local IFS='|'
    local items
    # Read into a simple indexed array (bash 3.2 compatible)
    set -- $pool
    local count=$#
    local idx=$(( RANDOM % count + 1 ))
    eval "echo \"\${$idx}\""
}

# pick_n - select N unique items from a |-delimited string, space-separated
# Usage: pick_n 3 "a|b|c|d|e"
pick_n() {
    local n="$1"
    local pool="$2"
    local IFS='|'
    set -- $pool
    local count=$#
    local result=""
    local used=""

    if [ "$n" -gt "$count" ]; then
        n=$count
    fi

    local picked=0
    local attempts=0
    while [ "$picked" -lt "$n" ] && [ "$attempts" -lt 50 ]; do
        local idx=$(( RANDOM % count + 1 ))
        eval "local item=\"\${$idx}\""
        attempts=$(( attempts + 1 ))
        # Check if already used
        case " $used " in
            *" $idx "*) continue ;;
        esac
        used="$used $idx"
        if [ -z "$result" ]; then
            result="$item"
        else
            result="$result $item"
        fi
        picked=$(( picked + 1 ))
    done
    echo "$result"
}

create_note() {
    local vault_path="$1"
    local filename="$2"
    local uuid="$3"
    local created="$4"
    local tags="$5"
    local content="$6"
    local parent="${7:-}"

    local parent_line=""
    if [ -n "$parent" ]; then
        parent_line="parent: $parent"
    fi

    if [ -n "$parent_line" ]; then
        cat > "$vault_path/$filename" <<EOF
---
uuid: $uuid
created: $created
updated: $created
tags: [$tags]
$parent_line
---

$content
EOF
    else
        cat > "$vault_path/$filename" <<EOF
---
uuid: $uuid
created: $created
updated: $created
tags: [$tags]
---

$content
EOF
    fi
}

create_vault() {
    local vault_path="$1"

    if [ -d "$vault_path" ]; then
        echo "Error: Directory already exists: $vault_path"
        echo "Use 'reset' to recreate or 'clean' first."
        exit 1
    fi

    echo "Creating test vault at: $vault_path"

    # Create vault and .ruin directory
    mkdir -p "$vault_path/.ruin"

    # --- Tag pools (|-delimited for pick()) ---
    local categories="daily|work|personal|idea|project|meeting|code|review|bug|design|infra|docs|platform"
    local projects="alpha|beta|infra|docs|platform"
    local statuses="draft|wip|done|blocked|urgent"

    # --- Content pools ---
    local daily_openings="Started the morning with coffee and planning.|Quiet start to the day, reviewing yesterday's progress.|Monday energy -- jumped straight into deep work.|Back from vacation, catching up on everything.|Rainy day, perfect for focused coding."
    local daily_tasks="Review open PRs|Update project board|Write weekly summary|Sync with team lead|Refactor auth module|Fix CI pipeline|Deploy staging build|Write unit tests|Update docs|Triage new issues"
    local daily_closings="Wrapped up early today, good progress overall.|Long day but productive. Need to follow up on the blocked items tomorrow.|Decent day. Left some loose ends for tomorrow.|Solid day of shipping. Feeling good about the sprint.|Ended with a few rabbit holes. Need to time-box better."

    local code_descriptions="Refactored the error handling in the API layer.|Added retry logic with exponential backoff.|Migrated the database schema for multi-tenancy.|Implemented streaming response for large payloads.|Fixed race condition in the worker pool.|Added circuit breaker to external service calls.|Optimized query performance with proper indexing.|Extracted shared logic into a reusable package.|Converted callbacks to async/await pattern.|Added structured logging with correlation IDs."
    local code_snippets="The key insight was using a channel-based approach instead of mutexes.|Using a decorator pattern here keeps the core logic clean.|The trick is to batch the writes and flush on a timer.|Interface segregation made testing much simpler.|Moved validation to the boundary -- fail fast, fail early."
    local code_todos="Add integration tests for the happy path|Benchmark the new implementation|Get code review from the platform team|Check if this breaks backward compat|Add metrics for the new endpoint"

    local meeting_topics="Sprint planning for Q2|Architecture review for new service|Incident postmortem -- outage on Feb 1|Cross-team sync on shared infrastructure|Onboarding plan for new hires|Dependency upgrade strategy|Performance budget discussion|Feature flag rollout plan|API versioning approach|Security audit findings"
    local meeting_attendees="Alice, Bob, Charlie|Dana, Eve, Frank|Grace, Hank|Ivy, Jake, Kim, Leo|Mona, Nick, Olivia"
    local meeting_actions="Follow up on the timeline estimate|Share the design doc with stakeholders|Schedule a deeper dive on caching|Create tickets for the agreed work|Update the runbook with new procedures"

    local idea_sparks="What if we exposed the CLI as a local HTTP API?|Could we auto-suggest tags based on content similarity?|Graph visualization of note relationships.|A weekly digest command that summarizes recent notes.|Plugin system for custom output formatters.|Fuzzy search with typo tolerance.|Note templates for common patterns.|Time-based note clustering.|Export to static site.|Integration with external knowledge bases."
    local idea_details="This would let other tools interact with the vault programmatically.|The embedding approach could work if we keep the index small.|Mermaid diagrams in the terminal might be too noisy -- maybe a web view.|Would need to think about privacy -- all local, no cloud.|Could use a simple scoring heuristic to start."

    local task_items="Set up monitoring for the new endpoint|Write migration script for legacy data|Review and merge the pending PRs|Update the deployment checklist|Add rate limiting to the public API|Create load test scenarios|Document the new config options|Fix the flaky test in CI|Audit third-party dependencies|Prepare demo for stakeholder review"

    # --- Hub notes (5 project hubs) ---
    local hub_projects="alpha|beta|infra|docs|platform"
    local hub_titles="Project Alpha Hub|Project Beta Hub|Infrastructure Hub|Documentation Hub|Platform Hub"
    local hub_descs="Central tracking note for Project Alpha. All alpha-related work links back here.|Project Beta coordination point. Design decisions and progress tracked below.|Infrastructure initiatives and operational improvements.|Documentation strategy, style guides, and content tracking.|Platform team hub -- shared services, libraries, and tooling."

    echo "  Creating 5 hub notes..."
    local hub_idx=0
    local IFS_SAVE="$IFS"
    IFS='|'
    set -- $hub_projects
    local hub_proj_arr=""
    local i=0
    for p in "$@"; do
        eval "hub_proj_$i=\"$p\""
        i=$(( i + 1 ))
    done
    IFS="$IFS_SAVE"

    IFS='|'
    set -- $hub_titles
    i=0
    for t in "$@"; do
        eval "hub_title_$i=\"$t\""
        i=$(( i + 1 ))
    done
    IFS="$IFS_SAVE"

    IFS='|'
    set -- $hub_descs
    i=0
    for d in "$@"; do
        eval "hub_desc_$i=\"$d\""
        i=$(( i + 1 ))
    done
    IFS="$IFS_SAVE"

    for hub_idx in 0 1 2 3 4; do
        eval "local proj=\"\$hub_proj_$hub_idx\""
        eval "local title=\"\$hub_title_$hub_idx\""
        eval "local desc=\"\$hub_desc_$hub_idx\""
        local hub_uuid="test-uuid-hub-$proj"
        local hub_created
        hub_created=$(date -v-30d "+%Y-%m-%dT09:00:00-08:00" 2>/dev/null || date -d "-30 days" "+%Y-%m-%dT09:00:00-08:00")
        local hub_fn
        hub_fn=$(echo "$title" | tr ' ' '-')".md"

        create_note "$vault_path" "$hub_fn" "$hub_uuid" "$hub_created" \
            "\"#project\", \"#$proj\"" \
            "# $title

#project #$proj

$desc

## Status

Actively maintained.

## Key Decisions

- Tracked in sub-notes linked via parent."
        echo "    $hub_fn ($hub_uuid)"
    done

    # --- Generate 100 regular notes ---
    echo "  Creating 100 notes..."

    local note_num=1
    while [ "$note_num" -le 100 ]; do
        local uuid
        uuid=$(printf "test-uuid-%03d" "$note_num")
        local days_ago=$(( RANDOM % 30 ))
        local hour=$(( RANDOM % 14 + 7 ))
        local minute=$(( RANDOM % 60 ))
        local created
        created=$(date -v-${days_ago}d -v${hour}H -v${minute}M "+%Y-%m-%dT%H:%M:%S-08:00" 2>/dev/null \
            || date -d "-${days_ago} days" "+%Y-%m-%dT%H:%M:%S-08:00")
        local date_part
        date_part=$(date -v-${days_ago}d "+%Y-%m-%d" 2>/dev/null \
            || date -d "-${days_ago} days" "+%Y-%m-%d")

        # Determine note type: 25 daily, 25 code, 20 meeting, 15 idea, 15 task
        local note_type
        if [ "$note_num" -le 25 ]; then
            note_type="daily"
        elif [ "$note_num" -le 50 ]; then
            note_type="code"
        elif [ "$note_num" -le 70 ]; then
            note_type="meeting"
        elif [ "$note_num" -le 85 ]; then
            note_type="idea"
        else
            note_type="task"
        fi

        # Pick tags
        local cat1 cat2 status proj_tag
        cat1=$(pick "$categories")
        cat2=$(pick "$categories")
        status=$(pick "$statuses")

        local parent=""
        local tags_str=""
        local content=""
        local filename=""
        local title=""

        case "$note_type" in
            daily)
                title="Daily Log $date_part"
                filename="Daily-Log-${date_part}-${note_num}.md"
                local opening task1 task2 task3 closing
                opening=$(pick "$daily_openings")
                task1=$(pick "$daily_tasks")
                task2=$(pick "$daily_tasks")
                task3=$(pick "$daily_tasks")
                closing=$(pick "$daily_closings")
                tags_str="\"#daily\", \"#$cat1\""
                if [ $(( RANDOM % 3 )) -eq 0 ]; then
                    tags_str="$tags_str, \"#$status\""
                fi
                content="# $title

#daily #$cat1

$opening

## Tasks
- $task1
- $task2
- $task3

## Notes

Focused on #$cat2 work today. Need to keep momentum.

$closing"
                ;;

            code)
                local proj
                proj=$(pick "$projects")
                title="Code - $(pick "$code_descriptions" | cut -c1-40)"
                filename="Code-Note-$(printf '%03d' $note_num).md"
                local desc snippet todo1 todo2
                desc=$(pick "$code_descriptions")
                snippet=$(pick "$code_snippets")
                todo1=$(pick "$code_todos")
                todo2=$(pick "$code_todos")
                tags_str="\"#code\", \"#$proj\""
                if [ $(( RANDOM % 2 )) -eq 0 ]; then
                    tags_str="$tags_str, \"#$status\""
                fi

                # ~40% of code notes get a hub parent
                if [ $(( RANDOM % 5 )) -lt 2 ]; then
                    parent="test-uuid-hub-$proj"
                fi

                content="# $title

#code #$proj

$desc

## Details

$snippet

## TODO
- $todo1
- $todo2

#$status"
                ;;

            meeting)
                local topic attendees action1 action2
                topic=$(pick "$meeting_topics")
                attendees=$(pick "$meeting_attendees")
                action1=$(pick "$meeting_actions")
                action2=$(pick "$meeting_actions")
                title="Meeting - $topic"
                filename="Meeting-$(printf '%03d' $note_num).md"
                tags_str="\"#meeting\", \"#$cat1\""
                if [ $(( RANDOM % 3 )) -eq 0 ]; then
                    tags_str="$tags_str, \"#meeting notes#\""
                fi

                content="# $title

#meeting #$cat1

## Attendees
$attendees

## Discussion

$topic

Discussed timelines and priorities. #work

## Action Items
- $action1
- $action2"
                ;;

            idea)
                local spark detail
                spark=$(pick "$idea_sparks")
                detail=$(pick "$idea_details")
                title="Idea - $spark"
                filename="Idea-$(printf '%03d' $note_num).md"
                tags_str="\"#idea\", \"#$cat1\""
                if [ $(( RANDOM % 2 )) -eq 0 ]; then
                    tags_str="$tags_str, \"#draft\""
                fi

                # Some ideas nest 2-3 levels deep under another idea
                # Notes 72, 74, 76 get parent = previous idea note
                if [ "$note_num" -eq 72 ]; then
                    parent="test-uuid-071"
                elif [ "$note_num" -eq 74 ]; then
                    parent="test-uuid-072"
                elif [ "$note_num" -eq 76 ]; then
                    parent="test-uuid-074"
                fi

                content="# $title

#idea #$cat1

$spark

## Thinking

$detail

## Next Steps

Think about this more. Maybe prototype something. #draft"
                ;;

            task)
                local t1 t2 t3 t4
                t1=$(pick "$task_items")
                t2=$(pick "$task_items")
                t3=$(pick "$task_items")
                t4=$(pick "$task_items")
                local proj
                proj=$(pick "$projects")
                title="Tasks - $proj ($date_part)"
                filename="Tasks-$(printf '%03d' $note_num).md"
                tags_str="\"#work\", \"#$proj\", \"#$status\""

                # ~40% of task notes get a hub parent
                if [ $(( RANDOM % 5 )) -lt 2 ]; then
                    parent="test-uuid-hub-$proj"
                fi

                content="# $title

#work #$proj #$status

## Open
- [ ] $t1
- [ ] $t2

## In Progress
- [x] $t3

## Done
- [x] $t4"
                ;;
        esac

        # --- Orphan parent tests (notes 98, 99, 100) ---
        if [ "$note_num" -eq 98 ]; then
            parent="test-uuid-orphan-parent-1"
        elif [ "$note_num" -eq 99 ]; then
            parent="test-uuid-orphan-parent-2"
        elif [ "$note_num" -eq 100 ]; then
            parent="test-uuid-orphan-parent-3"
        fi

        create_note "$vault_path" "$filename" "$uuid" "$created" "$tags_str" "$content" "$parent"

        if [ $(( note_num % 25 )) -eq 0 ]; then
            echo "    Created $note_num / 100 notes..."
        fi

        note_num=$(( note_num + 1 ))
    done

    # --- Write queries.yml ---
    echo "  Writing saved queries..."
    cat > "$vault_path/.ruin/queries.yml" <<'EOF'
queries:
  - name: daily-work
    query: "#daily && #work"
  - name: active-bugs
    query: "#bug && !#done"
  - name: project-alpha
    query: "#alpha"
  - name: urgent-items
    query: "#urgent || #blocked"
  - name: meeting-notes
    query: "#meeting"
EOF

    # --- Write parents.yml (saved parent bookmarks) ---
    echo "  Writing saved parent bookmarks..."
    cat > "$vault_path/.ruin/parents.yml" <<'EOF'
parents:
  - name: alpha
    uuid: test-uuid-hub-alpha
  - name: beta
    uuid: test-uuid-hub-beta
  - name: infra
    uuid: test-uuid-hub-infra
  - name: docs
    uuid: test-uuid-hub-docs
  - name: platform
    uuid: test-uuid-hub-platform
EOF

    # --- Run ruin doctor to build tags.yml and titles.json ---
    echo "  Running 'ruin doctor' to build indexes..."
    local ruin_bin=""
    # Try to find ruin binary
    if [ -x "./ruin" ]; then
        ruin_bin="./ruin"
    elif command -v ruin >/dev/null 2>&1; then
        ruin_bin="ruin"
    else
        echo "  WARNING: 'ruin' binary not found. Building..."
        local script_dir
        script_dir="$(cd "$(dirname "$0")" && pwd)"
        local project_dir
        project_dir="$(cd "$script_dir/.." && pwd)"
        if (cd "$project_dir" && go build -o ruin ./cmd/ruin) 2>/dev/null; then
            ruin_bin="$project_dir/ruin"
        else
            echo "  ERROR: Could not build ruin. Run 'make build' first, then re-run."
            echo "  Vault created but indexes are empty."
            echo ""
            echo "Done. Created 105 notes (5 hubs + 100 regular) at: $vault_path"
            echo "Test with: ./ruin --vault $vault_path log"
            return
        fi
    fi

    $ruin_bin --vault "$vault_path" doctor
    echo ""
    echo "Done. Created 105 notes (5 hubs + 100 regular) at: $vault_path"
    echo "  - 25 daily logs"
    echo "  - 25 code notes"
    echo "  - 20 meeting notes"
    echo "  - 15 idea notes"
    echo "  - 15 task lists"
    echo "  - 5 hub (project) notes"
    echo "  - 3 notes with orphaned parent references"
    echo "  - 5 saved queries in queries.yml"
    echo "  - 5 saved parent bookmarks (alpha, beta, infra, docs, platform)"
    echo ""
    echo "Test with:"
    echo "  ./ruin --vault $vault_path log"
    echo "  ./ruin --vault $vault_path search \"#daily\""
    echo "  ./ruin --vault $vault_path query list"
    echo "  ./ruin --vault $vault_path parent list"
    echo "  ./ruin --vault $vault_path compose alpha"
    echo "  ./ruin --vault $vault_path doctor --json"
}

clean_vault() {
    local vault_path="$1"

    if [ ! -d "$vault_path" ]; then
        echo "Directory does not exist: $vault_path"
        return 0
    fi

    # Safety check: only delete if .ruin/ directory exists
    if [ ! -d "$vault_path/.ruin" ]; then
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
