# Contributing to flint

Thank you for your interest in flint.

## Development

flint uses [Task](https://taskfile.dev) for common workflows.

```sh
task setup   # install deps + dev tools (golangci-lint, goimports, goreleaser)
task build   # build ./flint with version metadata
task test    # go test ./...
task lint    # golangci-lint
task ci      # fmt + vet + lint + test + build (run this before a PR)
```

Without Task, the equivalents are:

```sh
go build ./cmd/flint
go test ./...
go vet ./...
golangci-lint run
```

## Project layout

```
cmd/flint/            thin main() entrypoint
internal/cli/         Cobra commands and flag wiring
internal/awsconf/     AWS credential/region resolution (isolates the AWS SDK)
internal/awsprofiles/ profile discovery from ~/.aws config + credentials
internal/ui/          interactive profile picker (Charm: Bubble Tea, Lip Gloss)
internal/token/       Bedrock bearer-token derivation (SigV4 presign)
internal/logging/     slog-based diagnostic logger (stderr, secret-safe)
internal/version/     build-time version metadata
```

The AWS SDK stays inside `internal/awsconf`. The SigV4 signer stays inside
`internal/token`. Keep it that way, so the rest of the code stays easy to test.
The interactive TUI lives in `internal/ui`. Tests drive it headlessly with
Charm's `teatest`, so it needs no real terminal.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/), so GoReleaser
can generate the release changelog. Examples:

- `feat: add interactive profile picker`
- `fix: clamp duration before signing`
- `docs: clarify region-locking`

## Pull requests

Before you open a pull request, run `task ci` (or the manual commands) and make
sure it passes. Add tests for a behavior change. Both `internal/token` and
`internal/cli` have table-driven tests to extend.

## Releasing

To cut a release, push a `v*` tag. The `release` GitHub Actions workflow runs
GoReleaser. GoReleaser builds the cross-platform binaries and publishes the
Homebrew cask. First validate the configuration with `task release-check` and
`task release-snapshot`.
