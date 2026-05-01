# Changelog

## [Unreleased]

### Added
- Evalulate embeds dynamically with `ruin embed eval <embed-string>`, without needing to make a new document.
- Relative date arithmetic in filters and `@`-tokens: `today+N` and `today-N` (`created:today-7`, `between:today,today+6`).
- `ruin pick` between two dates with new `@between:D1,D2` argument, mirroring `search`
- Match all notes with no tags with new `tags:none` search filter.
- `tag_frontmatter` config flag (default `true`). Set to `false` to stop
  writing `tags:` and `inline-tags:` to note frontmatter — useful for
  users who want ruin to defer to Obsidian's tag-pane convention on a
  shared vault. `inherited-tags:` is unaffected. Settable via
  `~/.config/ruin/config.yml`, `RUIN_TAG_FRONTMATTER=false`, or
  `ruin config tag_frontmatter false`. See
  [`docs/obsidian-compatibility.md`](docs/obsidian-compatibility.md).

### Changed
- Frontmatter parsing preserves key order, YAML comments, and scalar quote styles for keys ruin doesn't manage.
- `ruin init` indexes folders with existing files with prompt
- **Breaking**: tag storage form changed. Stored tags drop the `#` prefix
  (`tags: [daily]` instead of `tags: ["#daily"]`). Affects frontmatter
  `tags:` / `inherited-tags:`, the `.ruin/tags.yml` index keys, and JSON
  output. CLI input (`ruin search "#daily"`) and body markdown (`#tag`,
  `#meeting notes#`) are unchanged. Migration is automatic on the first
  `ruin doctor` after upgrade and is recoverable via auto-version git
  history. See [`docs/breaking-changes/v0.4.0-tag-format.md`](docs/breaking-changes/v0.4.0-tag-format.md).
- **Breaking**: JSON output (`ruin search --json`, `ruin pick --json`,
  `ruin tags --json`, `ruin embed eval --json`, `ruin --bulk`) emits
  `tags`, `inline_tags`, and `inherited_tags` arrays without the `#`
  prefix. Downstream consumers must update.
- **Breaking**: `.ruin/` index files gain a per-file `version: 2` key
  (`tags.yml`, `queries.yml`, `parents.yml`, `titles.json`). Old binaries
  reading a v0.4.0 vault refuse with a clear "newer ruin required" error
  rather than silently giving wrong answers.

### Removed
- **Breaking**: `pkg/notetext.NormalizeTag` (and its `internal/note.NormalizeTag` shim) replaced by two clearer functions: `NormalizeStored` (storage form: lowercased, `#` delimiters stripped) for comparison and indexing, and `BodyForm` (lowercased, delimiters re-added based on whitespace) for emitting tags into note body content. Downstream Go consumers importing `pkg/notetext` must migrate calls.
- **Breaking**: `inline-tags:` field is now configurable. By default it is
  written to frontmatter in stripped form (no `#`); the new
  `tag_frontmatter=false` setting omits it (and `tags:`) entirely. In
  all modes the field is mirrored in `.ruin/titles.json` as the
  hot-path source of truth for matchers. `inherited-tags:` stays in
  frontmatter (in stripped form) as durable, user-visible metadata
  with its own titles.json mirror.
- The no-args `(*Frontmatter).Serialize` still omits `inline-tags:` for
  direct callers (the v0.4.0 default). Vault save paths use
  `SerializeWithOptions` (via `commands.saveNoteForVault`) and emit it
  when `tag_frontmatter=true` (the vault default).
- **Breaking**: `note.LoadFrontmatterOnly` no longer populates
  `Tags` / `InlineTags` / `InheritedTags` on the returned `*Note`. The
  `.ruin/titles.json` mirror is the source of truth for hot-path tag
  matching; downstream callers needing tags should query the titles
  index or call `note.Load` for body classification.

## [0.3.1] - 2026-04-21

### Fixed
- `@date` tokens inside inline code and fenced code blocks are no longer resolved or extracted
- Completed checkbox items are now considered `done` for the purposes of filtering and marked as so in the JSON output

## [0.3.0] - 2026-04-16

### Fixed
- Omit dynamic embed fencing unless using `--edit`
- Empty headers are omitted during dynamic embed
- Tags inside inline code and fenced code blocks are no longer extracted
- Checked checkbox lines (`- [x]`) are now reported as `done: true` in `pick --json` and are excluded by the default done filter, matching the existing `#done` tag semantics
- `--normalize-headers` now applies to dynamic embed output (`![[pick:]]`, `![[search:]]`)
- `compose --json` source maps are now complete for dynamic embeds

### Added
- `group` options in dynamic embeds
  - group by `parent`, `tag`, `root`, `note` (default). See [`compose-advanced.md`](docs/compose-advanced.md) for more information.
- `ruin log extract` subcommand — extract and classify tags from content without creating a note 
- `pkg/notetext` public Go package — tag extraction primitives importable by external tools
- Hyphens are now valid tag characters.

### Removed
- **Breaking**: YML compose files and file-based bookmarks removed
  - `ruin compose --file` / `-F` flag removed
  - `ruin parent save --file` flag removed
  - `![[]]` embeds and dynamic embeds cover the same use cases more naturally

## [0.2.1] - 2026-04-14

### Fixed
- `@today` and other date tokens inside `![[]]` embeds no longer resolve on save
- `![[pick:]]` embeds now handle `@date` tokens as line-level date filters

## [0.2.0] - 2026-04-14

### Added
- Support for negation via `!` for all terms in CLI
  - Ex. `#followup !#archive`, `meeting !title:draft`
- Support queries in `![[]]` syntax for dynamic embedding during `compose`,
  see [`compose-advanced.md`](docs/compose-advanced.md) for more information.

## [0.1.0] - 2026-04-08

First tagged prerelease. See [README.md](README.md) and [the cli reference](docs/cli-reference.md) for more information.

Breaking changes possible prior to 1.0. When necessary, they will be documented in the changelog and, where possible, a migration path will be provided via `ruin doctor`.
