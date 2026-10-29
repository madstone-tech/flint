package token

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpDoer is the minimal HTTP interface Verify needs, so tests can inject a
// fake without a real network call.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// LiveResult is the outcome of a live verification probe against Bedrock.
type LiveResult struct {
	// OK is true when Bedrock accepted the token (HTTP 2xx).
	OK bool
	// StatusCode is the HTTP status returned by the probe.
	StatusCode int
	// Detail carries the service's message on a non-2xx response (trimmed).
	Detail string
}

// Verify makes one lightweight, read-only Bedrock call (ListFoundationModels)
// with the bearer token to confirm that the service accepts it. region must
// match the region the token was signed for, or Bedrock rejects it. A nil
// client uses a short-timeout http.Client.
//
// Verify returns a LiveResult that describes the response. It returns a
// non-nil error only for transport failures, such as DNS, connection, or
// timeout. An authentication rejection is not an error. Verify reports a 403
// through LiveResult.OK set to false.
func Verify(ctx context.Context, client httpDoer, bearerToken, region string) (LiveResult, error) {
	if region == "" {
		return LiveResult{}, fmt.Errorf("region must not be empty")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	endpoint := fmt.Sprintf("https://bedrock.%s.amazonaws.com/foundation-models", region)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return LiveResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return LiveResult{}, fmt.Errorf("probe request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	res := LiveResult{StatusCode: resp.StatusCode}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		res.OK = true
		return res, nil
	}

	// Capture a bounded snippet of the error body for diagnostics.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	res.Detail = string(body)
	return res, nil
}
