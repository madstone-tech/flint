package token

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func testCreds() aws.Credentials {
	return aws.Credentials{ //nolint:gosec // G101: public example creds
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "SESSIONTOKEN123",
	}
}

func TestParseRoundTrip(t *testing.T) {
	tok, err := Generate(context.Background(), testCreds(), "us-east-1", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	p, err := Parse(tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if p.Region != "us-east-1" {
		t.Errorf("Region = %q, want us-east-1", p.Region)
	}
	if p.Action != "CallWithBearerToken" {
		t.Errorf("Action = %q, want CallWithBearerToken", p.Action)
	}
	if p.Expires != time.Hour {
		t.Errorf("Expires = %v, want 1h", p.Expires)
	}
	if p.IssuedAt.IsZero() {
		t.Error("IssuedAt should be set")
	}
	if !p.ExpiresAt.Equal(p.IssuedAt.Add(time.Hour)) {
		t.Errorf("ExpiresAt = %v, want IssuedAt+1h", p.ExpiresAt)
	}
}

func TestParseExpired(t *testing.T) {
	tok, err := Generate(context.Background(), testCreds(), "eu-west-1", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	p, err := Parse(tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Not expired now.
	if p.Expired(p.IssuedAt.Add(30 * time.Minute)) {
		t.Error("should not be expired 30m after issue")
	}
	// Expired well after the window.
	if !p.Expired(p.IssuedAt.Add(2 * time.Hour)) {
		t.Error("should be expired 2h after issue")
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	// Build a valid token to mutate.
	valid, err := Generate(context.Background(), testCreds(), "us-east-1", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"wrong prefix", "api-key-abc"},
		{"bad base64", authPrefix + "!!!not-base64!!!"},
		{
			"missing version suffix",
			authPrefix + base64.StdEncoding.EncodeToString([]byte(host+"/?Action=CallWithBearerToken")),
		},
		{
			"wrong host",
			authPrefix + base64.StdEncoding.EncodeToString([]byte("evil.example.com/?Action=x"+tokenSuffix)),
		},
		{
			"missing sigv4 params",
			authPrefix + base64.StdEncoding.EncodeToString([]byte(host+"/?Action=CallWithBearerToken"+tokenSuffix)),
		},
		{"truncated payload", valid[:len(valid)-10]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.token); err == nil {
				t.Errorf("expected Parse to reject %q", tt.name)
			}
		})
	}
}

func TestParseDetectsLongTermKey(t *testing.T) {
	// A long-term key is base64 whose payload contains "BedrockAPIKey-".
	longTerm := base64.StdEncoding.EncodeToString([]byte("\x00\x14BedrockAPIKey-acct-123:secretpart="))
	_, err := Parse(longTerm)
	if err == nil {
		t.Fatal("expected long-term key to be rejected")
	}
	if !strings.Contains(err.Error(), "long-term") {
		t.Errorf("error should mention long-term key, got: %v", err)
	}
}

func TestRegionFromCredential(t *testing.T) {
	ok := "AKID/20260101/ap-south-1/bedrock/aws4_request"
	region, err := regionFromCredential(ok)
	if err != nil || region != "ap-south-1" {
		t.Errorf("regionFromCredential(%q) = %q, %v", ok, region, err)
	}

	bad := []string{
		"too/few/parts",
		"AKID/20260101/us-east-1/s3/aws4_request",       // wrong service
		"AKID/20260101/us-east-1/bedrock/not_a_request", // wrong terminator
	}
	for _, c := range bad {
		if _, err := regionFromCredential(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParseDurationCapReflected(t *testing.T) {
	tok, err := Generate(context.Background(), testCreds(), "us-west-2", 48*time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	p, err := Parse(tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p.Expires != MaxDuration {
		t.Errorf("Expires = %v, want 12h cap", p.Expires)
	}
}

// Guard: a token for one region should not claim another.
func TestParseRegionMatchesGenerate(t *testing.T) {
	for _, region := range []string{"us-east-1", "eu-central-1", "ap-southeast-2"} {
		tok, err := Generate(context.Background(), testCreds(), region, time.Hour)
		if err != nil {
			t.Fatalf("Generate(%s): %v", region, err)
		}
		p, err := Parse(tok)
		if err != nil {
			t.Fatalf("Parse(%s): %v", region, err)
		}
		if p.Region != region {
			t.Errorf("Region = %q, want %q", p.Region, region)
		}
		if strings.Contains(tok, " ") {
			t.Errorf("token should not contain spaces")
		}
	}
}
