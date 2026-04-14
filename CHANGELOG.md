# Changelog

## [0.2.0] - 2026-04-14

### Added
- Support for negation via `!` for all terms in CLI
  - Ex. `#followup !#archive`, `meeting !title:draft`
- Support queries in `![[]]` syntax for dynamic embedding during `compose`,
  see [`compose-advanced.md`](docs/compose-advanced.md) for more information.

## [0.1.0] - 2026-04-08

First tagged prerelease. See [README.md](README.md) and [the cli reference](docs/cli-reference.md) for more information.

Breaking changes possible prior to 1.0. When necessary, they will be documented in the changelog and, where possible, a migration path will be provided via `ruin doctor`.
