# Changelog

## [0.2.2] - 2026-04-15

### Fixed
- Omit dynamic embed fencing unless using `--edit`
- Empty headers are omitted during dynamic embed

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
