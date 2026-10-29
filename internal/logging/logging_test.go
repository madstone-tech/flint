package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	cases := map[string]struct {
		want Format
		ok   bool
	}{
		"":     {FormatText, true},
		"text": {FormatText, true},
		"TEXT": {FormatText, true},
		"json": {FormatJSON, true},
		"yaml": {"", false},
	}
	for in, exp := range cases {
		got, ok := ParseFormat(in)
		if ok != exp.ok || (ok && got != exp.want) {
			t.Errorf("ParseFormat(%q) = (%q, %v), want (%q, %v)", in, got, ok, exp.want, exp.ok)
		}
	}

	// Surrounding whitespace is trimmed.
	if got, ok := ParseFormat("  json  "); !ok || got != FormatJSON {
		t.Errorf("ParseFormat with whitespace = (%q, %v), want (json, true)", got, ok)
	}
}

func TestNewVerboseWritesText(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, true, FormatText)
	l.Info("hello", "k", "v")

	out := buf.String()
	if !strings.Contains(out, "hello") || !strings.Contains(out, "k=v") {
		t.Errorf("text log missing content: %q", out)
	}
}

func TestNewVerboseWritesJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, true, FormatJSON)
	l.Info("hello", "k", "v")

	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) || !strings.Contains(out, `"k":"v"`) {
		t.Errorf("json log missing content: %q", out)
	}
}

func TestNewQuietDiscards(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, false, FormatText)
	l.Info("should not appear")
	l.Error("neither should this")
	if buf.Len() != 0 {
		t.Errorf("expected no output when not verbose, got %q", buf.String())
	}
}
