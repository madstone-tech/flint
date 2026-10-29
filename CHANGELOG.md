# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of `flint`.
- Derive temporary Amazon Bedrock bearer tokens from any AWS profile (static
  keys, SSO, assume-role, environment variables) by local SigV4
  query-presigning. flint makes no privileged API call.
- Output modes: shell `export` line (default), `--raw`, `--json`, and `--exec`
  to run a command with the token in its environment.
- `--region` and `--duration` flags. flint clamps the duration to a 5m-12h
  range.
- `--version` and the `version` command print build metadata.
- Interactive profile picker (Charm: Bubble Tea, Bubbles, Lip Gloss). flint
  shows it when you run `flint` with no profile argument in an interactive
  terminal. It supports a fuzzy filter, arrow-key navigation, Enter to select,
  and Esc to cancel. flint draws it on stderr, so `eval "$(flint)"` stays safe.
  Without a terminal, flint uses the default credential chain.
- `flint verify` validates a token in two tiers:
  - The structural check is offline. It confirms the prefix, base64 payload,
    version suffix, host, SigV4 parameters, region, and expiry window.
  - The `--live` flag probes Bedrock with `ListFoundationModels` to confirm
    acceptance.
  - It reads the token from an argument, from piped stdin, or from
    `AWS_BEARER_TOKEN_BEDROCK`. It also detects a long-term Bedrock API key and
    reports it clearly.
- `--verbose` diagnostic logging (through `log/slog`) to stderr, with
  `--log-format text|json`. flint is quiet by default. The logs carry only
  metadata (profile, region, expiry, output mode), never the token or secrets.
- Stable exit codes (0 ok, 1 generic, 2 usage, 3 credentials, 4 token
  rejected or expired, 130 cancelled). Errors print a one-line message with a
  `hint:` for the next step where it is useful.
- `flint shellenv` prints lazy shell helpers (`bedrock-auth`, `bedrock-run`) for
  `eval "$(flint shellenv)"` in a dotfile.
- `flint profiles` (alias `ls`) lists the discovered AWS profiles, one per line.
- `--exec` propagates the child command's exit code, so flint acts as a
  transparent wrapper. SIGINT and SIGTERM cancel in-flight credential calls and
  the child process.
- Shell completions (bash, zsh, fish) ship in the release archives. CI runs a
  linux, macOS, and windows test matrix and `govulncheck`.
