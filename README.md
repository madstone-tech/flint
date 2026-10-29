# flint

[![CI](https://github.com/madstone-tech/flint/actions/workflows/ci.yml/badge.svg)](https://github.com/madstone-tech/flint/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

flint is a small CLI that issues temporary Amazon Bedrock bearer tokens from an
AWS profile. It works like the `assume` command from
[granted.dev](https://granted.dev), but it issues a Bedrock API key instead of
raw AWS credentials.

## What flint does

A Bedrock short-term API key is not a server-issued resource. It is a
[SigV4 query-presigned request][api-keys] against `bedrock.amazonaws.com`.
To build the token, flint base64-encodes that request and adds the prefix
`bedrock-api-key-`. The token comes from the credentials that your profile
resolves to:

1. Resolve credentials through the standard AWS configuration chain. This chain
   supports static keys, SSO, `assume-role`, and environment variables.
2. SigV4 query-presign the `CallWithBearerToken` action for your region.
3. Encode the result into the `bedrock-api-key-...&Version=1` token format.

The token has these properties:

- It inherits the permissions of the resolved principal.
- It is region-locked. It works only for the region that flint signed it for.
- It is valid for the shortest of three values: `--duration`, 12 hours, and the
  time left on the credentials. flint raises `--duration` to a 5-minute floor.

flint makes no privileged "issue key" API call. It derives the token on the
client.

## Install

```sh
# Homebrew (macOS and Linux)
brew install --cask madstone-tech/tap/flint

# Go
go install github.com/madstone-tech/flint/cmd/flint@latest

# From source
git clone https://github.com/madstone-tech/flint.git
cd flint && task build   # or: go build -o flint ./cmd/flint
```

To confirm the install, run `flint --version`.

## Usage

The default output is a shell `export` line, so you can use it like `assume`:

```sh
eval "$(flint my-profile)"
# AWS_BEARER_TOKEN_BEDROCK is now set. The Bedrock CLI and SDKs read it.
aws bedrock-runtime converse --model-id ... --region us-east-1
```

Other modes:

```sh
flint --raw my-profile                     # print the token only
flint --json my-profile                    # print {"token":...,"region":...,"expires":...}
flint --region us-west-2 my-profile        # set the region
flint --duration 1h my-profile             # set a shorter lifetime (5m-12h)
flint --exec my-profile -- python app.py   # run a command with the token in its environment
```

### Interactive profile picker

Run `flint` with no profile argument in an interactive terminal. flint opens a
filterable picker. The picker lists the profiles from your `~/.aws/config` and
`~/.aws/credentials` files:

```sh
flint                 # pick a profile, then print the export line
eval "$(flint)"       # pick a profile and export the token in one step
```

To filter, type part of a profile name. Use the arrow keys to move, Enter to
select, and Esc to cancel.

flint draws the picker on stderr, so it does not corrupt the token on stdout.
`eval "$(flint)"` works even while the picker is on screen. flint shows the
picker only when stdin and stderr are both a terminal. Without a terminal (a
pipe, a script, or CI), flint skips the picker. In that case you must give a
profile argument, or flint uses the default credential chain (`AWS_PROFILE` and
environment variables).

For the full list of flags, run `flint --help`.

## Shell integration

Add flint's helpers to your `~/.bashrc` or `~/.zshrc`:

```sh
eval "$(flint shellenv)"
```

This defines two functions. It does not export a token at shell startup. An
eager export would freeze a token for up to 12 hours and run flint on every new
shell.

```sh
bedrock-auth my-profile                 # mint a token into the current shell, on demand
bedrock-run my-profile -- python app.py # run a command with the token in its environment only
```

To list the profiles that flint can see, run `flint profiles`. It prints one
profile per line from your `~/.aws/config` and `~/.aws/credentials` files.

## Logging and exit codes

flint is quiet by default. Pass `--verbose` for step-by-step diagnostic logs.
The logs cover credential resolution, the region, the duration limits, and the
output mode. flint writes the logs to stderr.

The logs never include the token value or any secret. They include only
metadata, such as the profile name, the region, and the expiry. For this reason,
`eval "$(flint --verbose my-profile)"` stays safe. For machine-readable logs,
pass `--log-format json`.

Commands return stable exit codes, so scripts can branch on a failure:

| Code | Meaning                                             |
|------|-----------------------------------------------------|
| 0    | success                                             |
| 1    | generic error                                       |
| 2    | bad invocation (flags or arguments, or no token to verify) |
| 3    | flint could not resolve credentials or a region     |
| 4    | the token is invalid, expired, or rejected          |
| 130  | the user cancelled (for example, the picker closed) |

An error prints a one-line message. Where it is useful, flint adds a `hint:`
line with the next step. For example, when a profile is not signed in, the hint
is `aws sso login --profile X`.

## Verifying a token

`flint verify` checks a token in two tiers.

The structural check runs offline and makes no AWS call. flint decodes the
token and confirms these parts:

- the `bedrock-api-key-` prefix
- the base64 payload
- the `&Version=1` version suffix
- the `bedrock.amazonaws.com` host
- the SigV4 parameters
- the region
- the signature window

```sh
flint --raw my-profile | flint verify
```

```
structure: valid
  region:    us-east-1
  action:    CallWithBearerToken
  issued:    2026-10-29T15:00:00Z
  expires:   2026-10-30T03:00:00Z (in 11h59m)
```

The live check adds `--live`. It calls `ListFoundationModels` with the token to
confirm that Bedrock accepts it. The region must match the region that flint
signed the token for:

```sh
flint --raw my-profile | flint verify --live
# live:      accepted by Bedrock (HTTP 200) in us-east-1
```

flint reads the token from the positional argument first, then from piped
stdin, then from the `AWS_BEARER_TOKEN_BEDROCK` environment variable. `verify`
exits non-zero when the token is invalid, expired, or rejected, so it works in
scripts. flint also detects a long-term Bedrock API key. In that case it tells
you that flint handles short-term tokens only.

AWS has no "validate token" API. flint can confirm the signature and the
principal's permissions only by a real call, which is what `--live` does.

## Project layout

```
cmd/flint/            thin main() entrypoint
internal/cli/         Cobra commands and flag wiring
internal/awsconf/     AWS credential and region resolution (isolates the AWS SDK)
internal/awsprofiles/ profile discovery from the ~/.aws config and credentials files
internal/ui/          interactive profile picker (Charm: Bubble Tea, Lip Gloss)
internal/token/       Bedrock bearer-token derivation (SigV4 presign)
internal/logging/     slog-based diagnostic logger (stderr, secret-safe)
internal/version/     build-time version metadata
```

## Development

```sh
task setup   # install dependencies and dev tools
task ci      # fmt, vet, lint, test, and build
```

For more detail, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Notes

- The Bedrock SDKs and the AWS CLI read the `AWS_BEARER_TOKEN_BEDROCK`
  environment variable.
- You cannot refresh or extend a token. When a token expires, generate a new
  one.
- Store tokens in a safe place and do not log them. `--exec` keeps the token out
  of your shell history.

## License

[MIT](LICENSE) © MADSTONE TECHNOLOGY™

## Trademark

flint and MADSTONE TECHNOLOGY™ are trademarks of MADSTONE TECHNOLOGY. The MIT
license covers the source code. It does not grant rights to use the MADSTONE
TECHNOLOGY name or marks.

[api-keys]: https://docs.aws.amazon.com/bedrock/latest/userguide/api-keys.html
