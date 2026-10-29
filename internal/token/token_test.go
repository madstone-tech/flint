package token

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestGenerateFormat(t *testing.T) {
	// These are AWS's well-known documentation example credentials, not real
	// secrets.
	creds := aws.Credentials{ //nolint:gosec // G101: public example creds
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "SESSIONTOKEN123",
	}

	tok, err := Generate(context.Background(), creds, "us-east-1", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.HasPrefix(tok, authPrefix) {
		t.Fatalf("missing prefix %q in %q", authPrefix, tok)
	}

	payload := strings.TrimPrefix(tok, authPrefix)
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("payload not valid base64: %v", err)
	}
	dec := string(decoded)

	if !strings.HasSuffix(dec, tokenSuffix) {
		t.Fatalf("missing version suffix %q in %q", tokenSuffix, dec)
	}
	if !strings.HasPrefix(dec, host+"/") {
		t.Fatalf("decoded token should start with host, got %q", dec)
	}

	// Parse the presigned URL and assert the expected SigV4 query params.
	presigned := strings.TrimSuffix(dec, tokenSuffix)
	u, err := url.Parse("https://" + presigned)
	if err != nil {
		t.Fatalf("parse presigned url: %v", err)
	}
	q := u.Query()
	for _, k := range []string{
		"Action", "X-Amz-Algorithm", "X-Amz-Credential",
		"X-Amz-Date", "X-Amz-Expires", "X-Amz-SignedHeaders", "X-Amz-Signature",
	} {
		if q.Get(k) == "" {
			t.Errorf("expected query param %q to be set", k)
		}
	}
	if got := q.Get("Action"); got != "CallWithBearerToken" {
		t.Errorf("Action = %q, want CallWithBearerToken", got)
	}
	if got := q.Get("X-Amz-Expires"); got != "3600" {
		t.Errorf("X-Amz-Expires = %q, want 3600", got)
	}
	if !strings.Contains(q.Get("X-Amz-Credential"), "/us-east-1/bedrock/aws4_request") {
		t.Errorf("credential scope wrong: %q", q.Get("X-Amz-Credential"))
	}
}

func TestDurationCap(t *testing.T) {
	creds := aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET"} //nolint:gosec // test placeholder
	tok, err := Generate(context.Background(), creds, "eu-west-1", 48*time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(tok, authPrefix))
	u, _ := url.Parse("https://" + strings.TrimSuffix(string(decoded), tokenSuffix))
	if got := u.Query().Get("X-Amz-Expires"); got != "43200" {
		t.Errorf("X-Amz-Expires = %q, want 43200 (12h cap)", got)
	}
}

func TestDurationFloor(t *testing.T) {
	creds := aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET"} //nolint:gosec // test placeholder
	// A sub-floor request (30s) must be raised to the 5m floor (300s), never
	// truncated to 0.
	tok, err := Generate(context.Background(), creds, "us-east-1", 30*time.Second)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(tok, authPrefix))
	u, _ := url.Parse("https://" + strings.TrimSuffix(string(decoded), tokenSuffix))
	if got := u.Query().Get("X-Amz-Expires"); got != "300" {
		t.Errorf("X-Amz-Expires = %q, want 300 (5m floor)", got)
	}
}

func TestClampDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want time.Duration
	}{
		{0, MaxDuration},                // unset → default 12h
		{-time.Hour, MaxDuration},       // negative → default 12h
		{48 * time.Hour, MaxDuration},   // over cap → 12h
		{time.Millisecond, MinDuration}, // sub-second → 5m floor
		{30 * time.Second, MinDuration}, // under floor → 5m
		{MinDuration, MinDuration},      // exactly floor → unchanged
		{time.Hour, time.Hour},          // in range → unchanged
		{MaxDuration, MaxDuration},      // exactly cap → unchanged
	}
	for _, tt := range tests {
		if got := ClampDuration(tt.in); got != tt.want {
			t.Errorf("ClampDuration(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
