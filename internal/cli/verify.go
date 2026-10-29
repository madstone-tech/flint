package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/madstone-tech/flint/internal/logging"
	"github.com/madstone-tech/flint/internal/token"
)

// verifyOptions holds the inputs for `flint verify`, decoupled from Cobra.
type verifyOptions struct {
	// Token is the bearer token to check. If empty, the caller is expected to
	// have already resolved it from stdin or the environment.
	Token string
	// Live runs the online probe against Bedrock in addition to structural
	// validation.
	Live bool
}

// liveVerifier performs the online probe. Injectable so tests avoid network.
type liveVerifier func(ctx context.Context, bearerToken, region string) (token.LiveResult, error)

// runVerify structurally validates the token and, when opts.Live is set, probes
// Bedrock. It writes a human-readable report to w and logs diagnostics to log.
// It returns an error when the token is structurally invalid, expired, or (in
// live mode) rejected, so the command exits non-zero for scripting.
func runVerify(ctx context.Context, w io.Writer, log *slog.Logger, opts verifyOptions, verify liveVerifier) error {
	if log == nil {
		log = logging.Discard()
	}
	tok := strings.TrimSpace(opts.Token)
	if tok == "" {
		return withExit(exitUsage,
			fmt.Sprintf("pass a token as an argument, pipe it on stdin, or set %s", envVar),
			fmt.Errorf("no token provided"))
	}

	log.Debug("parsing token structurally", "length", len(tok))
	parsed, err := token.Parse(tok)
	if err != nil {
		log.Error("structural validation failed", "error", err)
		return withExit(exitRejected, "", fmt.Errorf("structural validation failed: %w", err))
	}
	log.Info("structural validation passed",
		"region", parsed.Region,
		"action", parsed.Action,
		"expires", parsed.ExpiresAt.Format(time.RFC3339),
	)

	p := &printer{w: w}
	p.printf("structure: valid\n")
	p.printf("  region:    %s\n", parsed.Region)
	p.printf("  action:    %s\n", parsed.Action)
	p.printf("  issued:    %s\n", parsed.IssuedAt.Format(time.RFC3339))
	p.printf("  expires:   %s (in %s)\n", parsed.ExpiresAt.Format(time.RFC3339), until(parsed.ExpiresAt))

	if parsed.Expired(time.Now()) {
		p.printf("  status:    EXPIRED (signature window has passed)\n")
		log.Error("token expired", "expires", parsed.ExpiresAt.Format(time.RFC3339))
		if err := p.err(); err != nil {
			return err
		}
		return withExit(exitRejected, "mint a new token; short-term tokens cannot be refreshed",
			fmt.Errorf("token has expired"))
	}

	if !opts.Live {
		log.Debug("skipping live probe (--live not set)")
		return p.err()
	}

	if verify == nil {
		verify = defaultLiveVerifier
	}
	log.Debug("probing Bedrock", "region", parsed.Region)
	res, verr := verify(ctx, tok, parsed.Region)
	if verr != nil {
		log.Error("live probe failed", "region", parsed.Region, "error", verr)
		return withExit(exitGeneric, "check network connectivity and that the region is reachable",
			fmt.Errorf("live probe failed: %w", verr))
	}
	if res.OK {
		p.printf("live:      accepted by Bedrock (HTTP %d) in %s\n", res.StatusCode, parsed.Region)
		log.Info("token accepted by Bedrock", "status", res.StatusCode, "region", parsed.Region)
		return p.err()
	}

	p.printf("live:      REJECTED by Bedrock (HTTP %d)\n", res.StatusCode)
	if d := strings.TrimSpace(res.Detail); d != "" {
		p.printf("  detail:    %s\n", truncate(d, 300))
	}
	log.Error("token rejected by Bedrock", "status", res.StatusCode)
	if err := p.err(); err != nil {
		return err
	}
	return withExit(exitRejected, "confirm the region matches the token and the principal still has Bedrock access",
		fmt.Errorf("token rejected by Bedrock (HTTP %d)", res.StatusCode))
}

// printer accumulates the first write error so report output can be emitted
// without checking every call, while still surfacing a genuine I/O failure.
type printer struct {
	w       io.Writer
	firstEr error
}

func (p *printer) printf(format string, a ...any) {
	if p.firstEr != nil {
		return
	}
	if _, err := fmt.Fprintf(p.w, format, a...); err != nil {
		p.firstEr = err
	}
}

func (p *printer) err() error { return p.firstEr }

// until renders a friendly "time from now" for a future instant.
func until(t time.Time) string {
	d := time.Until(t).Round(time.Second)
	if d < 0 {
		return "expired"
	}
	return d.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
