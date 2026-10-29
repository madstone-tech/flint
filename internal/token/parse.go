package token

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Parsed is the decoded contents of a Bedrock bearer token. flint derives it
// only from the token string and makes no AWS call. It shows what the token
// claims, not whether AWS will accept it.
type Parsed struct {
	// Region the token is locked to (from the SigV4 credential scope).
	Region string
	// Action the presigned request authorizes. Expected: CallWithBearerToken.
	Action string
	// IssuedAt is the X-Amz-Date the request was signed at.
	IssuedAt time.Time
	// Expires is the requested validity window (X-Amz-Expires).
	Expires time.Duration
	// ExpiresAt is IssuedAt plus Expires, when the signature stops being valid.
	// The real lifetime is the shorter of this and the signing credentials'
	// own expiry, which the token does not encode.
	ExpiresAt time.Time
}

// Expired reports whether the token's signature window has passed as of now.
// This is a local check against the embedded timestamps. It does not account
// for the signing credentials' own expiry, which the token does not encode.
func (p Parsed) Expired(now time.Time) bool {
	return !p.ExpiresAt.IsZero() && now.After(p.ExpiresAt)
}

// Parse structurally validates a Bedrock bearer token and returns its decoded
// contents. It checks the prefix, base64 payload, version suffix, host, and the
// required SigV4 query parameters. It makes no network call. A successful
// Parse means the token is well-formed, not that AWS will accept it.
func Parse(tokenStr string) (Parsed, error) {
	if !strings.HasPrefix(tokenStr, authPrefix) {
		// A long-term Bedrock API key (from CreateServiceSpecificCredential) is
		// base64 that decodes to "BedrockAPIKey-...". Detect it to give a clear
		// message instead of a confusing prefix error.
		if looksLikeLongTermKey(tokenStr) {
			return Parsed{}, fmt.Errorf("this looks like a long-term Bedrock API key, not a short-term token; flint only validates short-term tokens (the %q format)", authPrefix)
		}
		return Parsed{}, fmt.Errorf("not a Bedrock bearer token: missing %q prefix", authPrefix)
	}
	payload := strings.TrimPrefix(tokenStr, authPrefix)

	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return Parsed{}, fmt.Errorf("token payload is not valid base64: %w", err)
	}
	dec := string(decoded)

	if !strings.HasSuffix(dec, tokenSuffix) {
		return Parsed{}, fmt.Errorf("token missing version suffix %q", tokenSuffix)
	}
	presigned := strings.TrimSuffix(dec, tokenSuffix)

	if !strings.HasPrefix(presigned, host+"/") {
		return Parsed{}, fmt.Errorf("token host is not %q", host)
	}

	u, err := url.Parse("https://" + presigned)
	if err != nil {
		return Parsed{}, fmt.Errorf("token is not a valid presigned URL: %w", err)
	}
	q := u.Query()

	// All SigV4 query-presign parameters must be present.
	for _, k := range []string{
		"Action", "X-Amz-Algorithm", "X-Amz-Credential",
		"X-Amz-Date", "X-Amz-Expires", "X-Amz-SignedHeaders", "X-Amz-Signature",
	} {
		if q.Get(k) == "" {
			return Parsed{}, fmt.Errorf("token missing required parameter %q", k)
		}
	}

	out := Parsed{Action: q.Get("Action")}

	// Region comes from the credential scope: <AKID>/<date>/<region>/<service>/aws4_request
	region, err := regionFromCredential(q.Get("X-Amz-Credential"))
	if err != nil {
		return Parsed{}, err
	}
	out.Region = region

	// X-Amz-Date uses ISO8601 basic format, for example 20260101T120000Z.
	issued, err := time.Parse("20060102T150405Z", q.Get("X-Amz-Date"))
	if err != nil {
		return Parsed{}, fmt.Errorf("token X-Amz-Date is malformed: %w", err)
	}
	out.IssuedAt = issued

	secs, err := strconv.Atoi(q.Get("X-Amz-Expires"))
	if err != nil {
		return Parsed{}, fmt.Errorf("token X-Amz-Expires is malformed: %w", err)
	}
	out.Expires = time.Duration(secs) * time.Second
	out.ExpiresAt = issued.Add(out.Expires)

	return out, nil
}

// regionFromCredential extracts the region from a SigV4 X-Amz-Credential value,
// whose scope is "<AccessKeyID>/<YYYYMMDD>/<region>/<service>/aws4_request".
func regionFromCredential(cred string) (string, error) {
	// url.Query already decodes the value. Split it on "/".
	parts := strings.Split(cred, "/")
	if len(parts) != 5 || parts[4] != "aws4_request" {
		return "", fmt.Errorf("X-Amz-Credential scope is malformed: %q", cred)
	}
	if parts[3] != serviceName {
		return "", fmt.Errorf("token is not scoped to the %q service (got %q)", serviceName, parts[3])
	}
	return parts[2], nil
}

// looksLikeLongTermKey reports whether a token is a long-term Bedrock API key
// (from CreateServiceSpecificCredential), whose base64 payload decodes to a
// string containing "BedrockAPIKey-".
func looksLikeLongTermKey(tokenStr string) bool {
	decoded, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		return false
	}
	return strings.Contains(string(decoded), "BedrockAPIKey-")
}
