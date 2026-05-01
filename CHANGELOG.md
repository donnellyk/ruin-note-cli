# Changelog

## [0.4.0] - 2026-05-01 

### Obsidian Compatibility
This release is focused on increasing compatibility with Obsidian, specifically tags. Ruin now stores and emits tags close to Obsidian's syntax (ie. drops the `#`). A new configuration `tag_frontmatter` (defaults to true) disables ruin writing body tags to a note's frontmatter, to closer align with Obsidian's frontmatter usage. See [`docs/breaking-changes/v0.4.0-tag-format.md`](docs/breaking-changes/v0.4.0-tag-format.md) and [`docs/obsidian-compatibility.md`](docs/obsidian-compatibility.md) for more information.

### New Command `ruin embed eval`
Evaluate embed strings dynamically without needing to make a new document.

### Added
- `ruin embed eval <embed-string>`
- Relative date arithmetic in filters and `@`-tokens: `today+N` and `today-N` (`created:today-7`, `between:today,today+6`).
- `ruin pick` between two dates with new `@between:D1,D2` argument, mirroring `search`
- Match all notes with no tags with new `tags:none` search filter.
- `tag_frontmatter` config flag

### Changed
- Tag-only Search / Pick performance improved by ~40%
- Frontmatter parsing now preserves key order, YAML comments, and scalar quote styles for keys ruin doesn't manage.
- `ruin init` indexes folders with existing files with prompt
- Stored tags drop `#` prefix
- JSON output emit tag arrayed without `#` prefix
- `.ruin/` index files now have `version` key
- `titles.yml` now stores tag information

### Fixed
- `compose --expand-embeds` no longer duplicates the results of a dynamic embed 

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
