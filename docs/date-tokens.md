# Date Tokens

Date tokens use the `@` prefix to reference dates in note content. They support task management workflows like due dates, follow-ups, and reminders.

## Syntax

| Token | Resolves to | Example |
|-------|-------------|---------|
| `@today` | Current date | `@2026-02-12` |
| `@tomorrow` | Next day | `@2026-02-13` |
| `@yesterday` | Previous day | `@2026-02-11` |
| `@monday` .. `@sunday` | Next occurrence (today if it matches) | `@2026-02-16` |
| `@next-week` | Next Monday | `@2026-02-16` |
| `@next-month` | 1st of next month | `@2026-03-01` |
| `@next-year` | Jan 1 of next year | `@2027-01-01` |
| `@2-days` | 2 days from now | `@2026-02-14` |
| `@3-weeks` | 3 weeks from now | `@2026-03-05` |
| `@2-months` | 2 months from now | `@2026-04-12` |
| `@2-years` | 2 years from now | `@2028-02-12` |
| `@2026-02-13` | Literal (unchanged) | `@2026-02-13` |

Unrecognized tokens (e.g., `@kevin`, `@deprecated`) are left unchanged. Email addresses (`user@company.com`) are not affected.

## Resolution

Relative date tokens are resolved to `@YYYY-MM-DD` format at **save time**. Once resolved, the literal date is stored in the note body permanently.

**Before save:**
```
Follow up with client @tomorrow #followup
```

**After save (if today is 2026-02-12):**
```
Follow up with client @2026-02-13 #followup
```

Re-saving a note does not re-resolve already-resolved dates.

## Frontmatter

All `@YYYY-MM-DD` dates found in the note body are extracted into a `dates` frontmatter field:

```yaml
---
uuid: abc-123
created: 2026-02-12T10:00:00-08:00
updated: 2026-02-12T10:00:00-08:00
inline-tags:
  - "#followup"
dates:
  - "2026-02-13"
---
```

The `dates` field is:
- Automatically managed by the CLI (rebuilt from content on every save)
- Sorted chronologically
- Deduplicated
- Used for fast query matching without loading full note content

## Querying

Use `@` tokens in search queries to find notes by referenced date:

```bash
# Find notes with dates matching tomorrow
ruin search "@tomorrow"

# Combine with tags
ruin search "#followup @tomorrow"

# Exact date match
ruin search "@2026-02-13"
```

Date tokens in queries are resolved dynamically at search time, so `@tomorrow` always matches notes with tomorrow's date regardless of when they were created.

Date queries match against the `dates` frontmatter field (fast path, no body scan required).

## Date Filters vs Date Tokens

These are complementary features:

| Feature | Syntax | Matches against | Purpose |
|---------|--------|-----------------|---------|
| Date tokens | `@today` | Dates in note body (`dates` field) | "What dates does this note reference?" |
| Date filters | `created:today` | Note metadata timestamps | "When was this note created/updated?" |

They can be combined:

```bash
# Notes created today that reference next week
ruin search "created:today @next-week"
```

## Supported Commands

Date tokens are resolved during:
- `ruin log` — when creating new notes
- `ruin search --edit` — when saving edited notes
- `ruin update` — when applying bulk changes
- `ruin doctor` — during vault repair/reindex
