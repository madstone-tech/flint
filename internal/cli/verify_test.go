package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awstoken "github.com/aws/aws-sdk-go-v2/aws"

	"github.com/madstone-tech/flint/internal/token"
)

// freshToken mints a real, structurally-valid token for tests.
func freshToken(t *testing.T, region string) string {
	t.Helper()
	tok, err := token.Generate(context.Background(), awstoken.Credentials{ //nolint:gosec // G101: public AWS example creds
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "SESSIONTOKEN",
	}, region, time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return tok
}

func TestRunVerifyStructuralOnly(t *testing.T) {
	var buf bytes.Buffer
	tok := freshToken(t, "us-east-1")

	err := runVerify(context.Background(), &buf, nil, verifyOptions{Token: tok}, nil)
	if err != nil {
		t.Fatalf("runVerify: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "structure: valid") {
		t.Errorf("missing structure line:\n%s", out)
	}
	if !strings.Contains(out, "region:    us-east-1") {
		t.Errorf("missing region:\n%s", out)
	}
	// No --live: should not mention a live probe.
	if strings.Contains(out, "live:") {
		t.Errorf("structural-only run should not probe:\n%s", out)
	}
}

func TestRunVerifyRejectsGarbage(t *testing.T) {
	err := runVerify(context.Background(), &bytes.Buffer{}, nil, verifyOptions{Token: "not-a-token"}, nil)
	if err == nil {
		t.Fatal("expected structural validation to fail")
	}
}

func TestRunVerifyEmptyToken(t *testing.T) {
	err := runVerify(context.Background(), &bytes.Buffer{}, nil, verifyOptions{Token: "   "}, nil)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestRunVerifyLiveAccepted(t *testing.T) {
	var buf bytes.Buffer
	tok := freshToken(t, "eu-west-1")

	var gotTok, gotRegion string
	verify := func(_ context.Context, bearer, region string) (token.LiveResult, error) {
		gotTok, gotRegion = bearer, region
		return token.LiveResult{OK: true, StatusCode: 200}, nil
	}

	err := runVerify(context.Background(), &buf, nil, verifyOptions{Token: tok, Live: true}, verify)
	if err != nil {
		t.Fatalf("runVerify: %v", err)
	}
	if gotTok != tok || gotRegion != "eu-west-1" {
		t.Errorf("verifier got (%q, %q), want the token and its region", gotTok, gotRegion)
	}
	if !strings.Contains(buf.String(), "accepted by Bedrock") {
		t.Errorf("missing acceptance line:\n%s", buf.String())
	}
}

func TestRunVerifyLiveRejected(t *testing.T) {
	var buf bytes.Buffer
	tok := freshToken(t, "us-east-1")

	verify := func(_ context.Context, _, _ string) (token.LiveResult, error) {
		return token.LiveResult{OK: false, StatusCode: 403, Detail: "not authorized"}, nil
	}

	err := runVerify(context.Background(), &buf, nil, verifyOptions{Token: tok, Live: true}, verify)
	if err == nil {
		t.Fatal("expected a rejection to return an error")
	}
	if !strings.Contains(buf.String(), "REJECTED by Bedrock (HTTP 403)") {
		t.Errorf("missing rejection line:\n%s", buf.String())
	}
}

func TestRunVerifyLiveProbeError(t *testing.T) {
	tok := freshToken(t, "us-east-1")
	boom := errors.New("connection refused")
	verify := func(_ context.Context, _, _ string) (token.LiveResult, error) {
		return token.LiveResult{}, boom
	}

	err := runVerify(context.Background(), &bytes.Buffer{}, nil, verifyOptions{Token: tok, Live: true}, verify)
	if !errors.Is(err, boom) {
		t.Errorf("expected probe error to propagate, got %v", err)
	}
}
