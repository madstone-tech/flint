// Command flint issues temporary Amazon Bedrock bearer tokens from an AWS
// profile, in the spirit of granted.dev's `assume`.
//
// A Bedrock short-term API key is not a server-issued resource: it is a SigV4
// query-presigned request against bedrock.amazonaws.com, base64-encoded and
// prefixed. flint derives it locally from whatever credentials your profile
// resolves to (static, SSO, assume-role, env), then exports it as
// AWS_BEARER_TOKEN_BEDROCK so the Bedrock SDKs and AWS CLI pick it up.
package main

import "github.com/madstone-tech/flint/internal/cli"

func main() {
	cli.Execute()
}
