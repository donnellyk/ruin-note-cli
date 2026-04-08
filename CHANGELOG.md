# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Pre-1.0 breaking changes bump the MINOR version and are documented under the
`Changed` heading with a link to [`docs/breaking-changes.md`](docs/breaking-changes.md) when applicable.

## [0.1.0] - 2026-04-08

First tagged release.

### Added
- Homebrew tap distribution via `donnellyk/homebrew-ruin`. Install with
  `brew install donnellyk/ruin/ruin-cli`.
- Prebuilt `darwin`/`linux` binaries (`amd64` + `arm64`) attached to GitHub releases.
- `CHANGELOG.md` as the source of truth for release notes; the release workflow
  extracts the current tag's section and publishes it as the GitHub release body.
- CI workflow (`.github/workflows/ci.yml`) running `go fmt`, `go vet`, and
  `go test ./...` on push and pull request to `main`.
- Release workflow (`.github/workflows/release.yml`) triggered by `v*` tags and
  driven by [GoReleaser](https://goreleaser.com).
- README badges: CI status, latest release, Go version, license.
- README install instructions covering Homebrew, `go install`, and local checkout.

### Changed
- Go module path changed from `kvnd/ruin-note-cli` to
  `github.com/donnellyk/ruin-note-cli`. This enables
  `go install github.com/donnellyk/ruin-note-cli/cmd/ruin@latest` as a secondary
  install method. All internal imports updated.

### Removed
- Windows target removed from `mise.toml` `build-all`. Windows is not a
  supported platform for v0.1.0 — no reliable testing infrastructure.
