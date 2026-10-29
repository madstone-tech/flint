// Package token derives short-term Amazon Bedrock bearer tokens from AWS
// credentials. A Bedrock short-term API key is not a server-issued resource.
// It is a SigV4 query-presigned request against bedrock.amazonaws.com. flint
// base64-encodes it and adds a prefix. This matches the reference
// implementation in aws/aws-bedrock-token-generator, so Bedrock accepts the
// tokens that flint produces.
package token

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const (
	host        = "bedrock.amazonaws.com"
	endpointURL = "https://" + host + "/"
	serviceName = "bedrock"
	authPrefix  = "bedrock-api-key-"
	tokenSuffix = "&Version=1"

	// MaxDuration is the hard cap Bedrock enforces on short-term keys.
	MaxDuration = 12 * time.Hour

	// MinDuration is flint's usability floor. A token's X-Amz-Expires window
	// starts at signing time, so a very short window can expire before the
	// request even reaches Bedrock (and is prone to SigV4 clock-skew flakiness).
	// 5 minutes mirrors AWS's common presign default and leaves ample headroom.
	MinDuration = 5 * time.Minute
)

// ClampDuration applies flint's duration policy: a non-positive or over-cap
// request becomes MaxDuration (12h), and anything below MinDuration (5m) is
// raised to the floor. The result always satisfies MinDuration <= d <= MaxDuration.
func ClampDuration(d time.Duration) time.Duration {
	switch {
	case d <= 0 || d > MaxDuration:
		return MaxDuration
	case d < MinDuration:
		return MinDuration
	default:
		return d
	}
}

// Generate derives a Bedrock bearer token from the supplied credentials for the
// given region. The returned token is valid for min(expires, MaxDuration, and
// the lifetime of the credentials themselves). Region-locked: the token only
// works against the region it was signed for.
func Generate(ctx context.Context, creds aws.Credentials, region string, expires time.Duration) (string, error) {
	if region == "" {
		return "", fmt.Errorf("region must not be empty")
	}
	expires = ClampDuration(expires)

	// POST https://bedrock.amazonaws.com/?Action=CallWithBearerToken&X-Amz-Expires=...
	req, err := newPresignRequest(expires)
	if err != nil {
		return "", err
	}

	signer := v4.NewSigner()
	// PresignHTTP produces the X-Amz-* query parameters (query-style SigV4),
	// identical to botocore's SigV4QueryAuth. emptyPayloadHash is SHA-256 of an
	// empty body, matching a POST with no body.
	signedURL, _, err := signer.PresignHTTP(
		ctx,
		creds,
		req,
		emptyPayloadHash,
		serviceName,
		region,
		time.Now(),
	)
	if err != nil {
		return "", fmt.Errorf("presigning bedrock request: %w", err)
	}

	// Mirror botocore: strip scheme, append version, base64 (standard) encode,
	// then prefix.
	presigned := strings.TrimPrefix(signedURL, "https://") + tokenSuffix
	encoded := base64.StdEncoding.EncodeToString([]byte(presigned))
	return authPrefix + encoded, nil
}
