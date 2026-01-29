#!/bin/bash
#
# Create benchmark vaults with realistic note distributions
# Usage: ./scripts/create-benchmark-vault.sh <size> [output-dir]
#   size: small (100), medium (1000), large (10000), xlarge (50000)
#

set -e

SIZE="${1:-medium}"
OUTPUT_DIR="${2:-/tmp/ruin-benchmark-vault}"

case "$SIZE" in
    small)  NUM_NOTES=100 ;;
    medium) NUM_NOTES=1000 ;;
    large)  NUM_NOTES=10000 ;;
    xlarge) NUM_NOTES=50000 ;;
    *)
        echo "Usage: $0 <small|medium|large|xlarge> [output-dir]"
        exit 1
        ;;
esac

echo "Creating $SIZE benchmark vault ($NUM_NOTES notes) at $OUTPUT_DIR..."

# Clean and create directory
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR/.ruin"

# Initialize .ruin directory
cat > "$OUTPUT_DIR/.ruin/tags.yml" << 'EOF'
tags: []
EOF

cat > "$OUTPUT_DIR/.ruin/queries.yml" << 'EOF'
queries: []
EOF

# Tag pools
TAGS_COMMON=("#daily" "#work" "#personal" "#todo" "#idea")
TAGS_CONTEXT=("#meeting" "#project" "#reference" "#draft" "#blog" "#notes")
TAGS_SPACED=("#meeting notes#" "#action items#" "#follow up#")

# Content blocks
PARAGRAPH="Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris. "

# Generate notes
for i in $(seq 0 $((NUM_NOTES - 1))); do
    # Vary creation dates across last 365 days
    DAYS_AGO=$((RANDOM % 365))
    HOUR=$((RANDOM % 24))
    MINUTE=$((RANDOM % 60))

    # macOS date command
    if [[ "$OSTYPE" == "darwin"* ]]; then
        CREATED=$(date -v-${DAYS_AGO}d -v${HOUR}H -v${MINUTE}M "+%Y-%m-%dT%H:%M:%S-05:00")
        DATE_PART=$(date -v-${DAYS_AGO}d "+%Y-%m-%d")
    else
        CREATED=$(date -d "-${DAYS_AGO} days" "+%Y-%m-%dT%H:%M:%S-05:00")
        DATE_PART=$(date -d "-${DAYS_AGO} days" "+%Y-%m-%d")
    fi

    # Determine note type based on distribution
    # 40% tiny, 30% small, 20% medium, 10% large
    TYPE_ROLL=$((i % 100))

    # Select tags (60% have 1-2, 30% have 3-5, 10% have 6+)
    TAG_ROLL=$((RANDOM % 100))
    if [ $TAG_ROLL -lt 60 ]; then
        NUM_TAGS=$((1 + RANDOM % 2))
    elif [ $TAG_ROLL -lt 90 ]; then
        NUM_TAGS=$((3 + RANDOM % 3))
    else
        NUM_TAGS=$((6 + RANDOM % 3))
    fi

    # Build tag list
    TAGS_YAML=""
    TAGS_CONTENT=""
    for t in $(seq 1 $NUM_TAGS); do
        if [ $((RANDOM % 10)) -lt 7 ]; then
            TAG="${TAGS_COMMON[$((RANDOM % ${#TAGS_COMMON[@]}))]}"
        elif [ $((RANDOM % 10)) -lt 9 ]; then
            TAG="${TAGS_CONTEXT[$((RANDOM % ${#TAGS_CONTEXT[@]}))]}"
        else
            TAG="${TAGS_SPACED[$((RANDOM % ${#TAGS_SPACED[@]}))]}"
        fi
        TAGS_YAML="$TAGS_YAML
  - \"$TAG\""
        TAGS_CONTENT="$TAGS_CONTENT $TAG"
    done

    # Generate UUID
    UUID=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo "uuid-$i")
    UUID=$(echo "$UUID" | tr '[:upper:]' '[:lower:]')

    # Create note content based on type
    if [ $TYPE_ROLL -lt 40 ]; then
        # Tiny note (~100 bytes) - quick thought
        TITLE="Quick thought $i"
        CONTENT="$TAGS_CONTENT

Remember this for later."
        FILENAME="thought-$(printf '%05d' $i).md"

    elif [ $TYPE_ROLL -lt 70 ]; then
        # Small note (~500 bytes) - daily note
        TITLE="Daily Note $DATE_PART"
        CONTENT="$TAGS_CONTENT

## Tasks
- Task item one
- Task item two
- Task item three

## Notes
$PARAGRAPH"
        FILENAME="daily-$(printf '%05d' $i).md"

    elif [ $TYPE_ROLL -lt 90 ]; then
        # Medium note (~2KB) - meeting notes
        TITLE="Meeting Notes $i"
        CONTENT="$TAGS_CONTENT

## Attendees
- Alice
- Bob
- Charlie
- Diana

## Agenda
1. Project status update
2. Q1 planning
3. Resource allocation

## Discussion
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH

## Action Items
- [ ] Follow up with stakeholders
- [ ] Send summary email
- [ ] Schedule follow-up meeting
- [ ] Update project timeline

## Notes
$PARAGRAPH
$PARAGRAPH"
        FILENAME="meeting-$(printf '%05d' $i).md"

    else
        # Large note (~10KB) - document
        TITLE="Document $i"
        CONTENT="$TAGS_CONTENT

## Introduction
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH

## Background
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH

## Main Content

### Section 1
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH

### Section 2
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH

### Section 3
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH

## Analysis
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH

## Conclusion
$PARAGRAPH
$PARAGRAPH
$PARAGRAPH

## References
- Reference 1
- Reference 2
- Reference 3
- Reference 4
- Reference 5"
        FILENAME="document-$(printf '%05d' $i).md"
    fi

    # Write note file
    cat > "$OUTPUT_DIR/$FILENAME" << EOF
---
uuid: $UUID
created: $CREATED
updated: $CREATED
tags:$TAGS_YAML
---
# $TITLE
$CONTENT
EOF

    # Progress indicator
    if [ $((i % 500)) -eq 0 ] && [ $i -gt 0 ]; then
        echo "  Created $i notes..."
    fi
done

echo "Created $NUM_NOTES notes in $OUTPUT_DIR"

# Show stats
echo ""
echo "Vault statistics:"
echo "  Total notes: $(ls -1 "$OUTPUT_DIR"/*.md 2>/dev/null | wc -l | tr -d ' ')"
echo "  Total size: $(du -sh "$OUTPUT_DIR" | cut -f1)"
echo "  Tiny notes: $(ls -1 "$OUTPUT_DIR"/thought-*.md 2>/dev/null | wc -l | tr -d ' ')"
echo "  Small notes: $(ls -1 "$OUTPUT_DIR"/daily-*.md 2>/dev/null | wc -l | tr -d ' ')"
echo "  Medium notes: $(ls -1 "$OUTPUT_DIR"/meeting-*.md 2>/dev/null | wc -l | tr -d ' ')"
echo "  Large notes: $(ls -1 "$OUTPUT_DIR"/document-*.md 2>/dev/null | wc -l | tr -d ' ')"
