package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/madstone-tech/flint/internal/logging"
)

func TestRunTokenLogsStepsWithoutLeakingToken(t *testing.T) {
	var out, logs bytes.Buffer
	log := logging.New(&logs, true, logging.FormatText)

	d := fakeDeps("us-east-1")
	d.log = log

	err := runToken(context.Background(), &out, tokenOptions{Raw: true, Duration: time.Hour}, d)
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}

	logStr := logs.String()
	// Explicit step logging is present.
	for _, want := range []string{"credentials resolved", "token minted", "region=us-east-1", "mode=raw"} {
		if !strings.Contains(logStr, want) {
			t.Errorf("log missing %q\n%s", want, logStr)
		}
	}

	// The token value must never appear in the logs.
	tok := strings.TrimSpace(out.String())
	if tok == "" {
		t.Fatal("expected a token on stdout")
	}
	if strings.Contains(logStr, tok) {
		t.Error("token value leaked into logs")
	}
	if strings.Contains(logStr, "bedrock-api-key-") {
		t.Error("token prefix leaked into logs")
	}
}

func TestRunTokenQuietByDefault(t *testing.T) {
	var out, logs bytes.Buffer
	// verbose=false → discard logger.
	log := logging.New(&logs, false, logging.FormatText)

	d := fakeDeps("eu-west-1")
	d.log = log

	if err := runToken(context.Background(), &out, tokenOptions{Raw: true}, d); err != nil {
		t.Fatalf("runToken: %v", err)
	}
	if logs.Len() != 0 {
		t.Errorf("expected no logs when verbose is off, got:\n%s", logs.String())
	}
}
