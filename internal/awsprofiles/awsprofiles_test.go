package awsprofiles

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestScanReaderSections(t *testing.T) {
	in := `
[default]
region = us-east-1

[profile dev]
region = us-west-2

; a comment
[profile prod]
`
	var got []string
	err := scanReader(strings.NewReader(in), func(s string) {
		got = append(got, s)
	})
	if err != nil {
		t.Fatalf("scanReader: %v", err)
	}
	want := []string{"default", "profile dev", "profile prod"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections = %v, want %v", got, want)
	}
}

func TestListMergesAndDedupes(t *testing.T) {
	dir := t.TempDir()

	configFile := filepath.Join(dir, "config")
	credsFile := filepath.Join(dir, "credentials")

	writeFile(t, configFile, `
[default]
region = us-east-1

[profile dev]
region = us-west-2

[profile shared]
`)
	writeFile(t, credsFile, `
[prod]
aws_access_key_id = AKID

[shared]
aws_access_key_id = AKID2
`)

	got, err := list(configFile, credsFile)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// Sorted + de-duplicated (shared appears in both files).
	want := []string{"default", "dev", "prod", "shared"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("list = %v, want %v", got, want)
	}
}

func TestListMissingFilesIsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := list(filepath.Join(dir, "nope-config"), filepath.Join(dir, "nope-creds"))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no profiles, got %v", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
