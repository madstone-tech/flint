package token

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeDoer returns a canned response (or error) and records the request it saw.
type fakeDoer struct {
	resp *http.Response
	err  error
	got  *http.Request
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.got = req
	return f.resp, f.err
}

func mkResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestVerifyAccepted(t *testing.T) {
	f := &fakeDoer{resp: mkResp(200, `{"modelSummaries":[]}`)} //nolint:bodyclose // Verify closes the body

	res, err := Verify(context.Background(), f, "bedrock-api-key-xyz", "us-east-1")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !res.OK || res.StatusCode != 200 {
		t.Errorf("result = %+v, want OK/200", res)
	}

	// Correct endpoint, method, and Authorization header.
	if f.got.URL.String() != "https://bedrock.us-east-1.amazonaws.com/foundation-models" {
		t.Errorf("endpoint = %q", f.got.URL.String())
	}
	if f.got.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", f.got.Method)
	}
	if got := f.got.Header.Get("Authorization"); got != "Bearer bedrock-api-key-xyz" {
		t.Errorf("Authorization = %q", got)
	}
}

func TestVerifyRejected(t *testing.T) {
	f := &fakeDoer{resp: mkResp(403, `{"message":"not authorized"}`)} //nolint:bodyclose // Verify closes the body

	res, err := Verify(context.Background(), f, "tok", "eu-west-1")
	if err != nil {
		t.Fatalf("Verify should not error on 403: %v", err)
	}
	if res.OK {
		t.Error("403 should not be OK")
	}
	if res.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", res.StatusCode)
	}
	if !strings.Contains(res.Detail, "not authorized") {
		t.Errorf("Detail = %q, want service message", res.Detail)
	}
}

func TestVerifyTransportError(t *testing.T) {
	f := &fakeDoer{err: io.ErrUnexpectedEOF}

	_, err := Verify(context.Background(), f, "tok", "us-east-1")
	if err == nil {
		t.Error("expected transport error to propagate")
	}
}

func TestVerifyRequiresRegion(t *testing.T) {
	if _, err := Verify(context.Background(), &fakeDoer{}, "tok", ""); err == nil {
		t.Error("expected error for empty region")
	}
}
