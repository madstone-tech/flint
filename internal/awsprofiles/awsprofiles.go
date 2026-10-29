// Package awsprofiles discovers AWS profile names from the shared config and
// credentials files. It is deliberately SDK-free and filesystem-driven so it
// can be unit-tested with temporary files.
package awsprofiles

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// List returns the sorted, de-duplicated set of profile names found in the
// user's AWS config and credentials files. The paths honor the AWS_CONFIG_FILE
// and AWS_SHARED_CREDENTIALS_FILE environment variables. They fall back to
// ~/.aws/config and ~/.aws/credentials.
func List() ([]string, error) {
	return list(configPath(), credentialsPath())
}

func list(configFile, credsFile string) ([]string, error) {
	seen := map[string]struct{}{}

	// In the config file, a section looks like [profile foo]. The default
	// profile is [default].
	if err := scan(configFile, func(section string) {
		name := strings.TrimSpace(strings.TrimPrefix(section, "profile "))
		if name != "" {
			seen[name] = struct{}{}
		}
	}); err != nil {
		return nil, err
	}

	// In the credentials file, a section is the raw profile name, such as [foo].
	if err := scan(credsFile, func(section string) {
		if section != "" {
			seen[section] = struct{}{}
		}
	}); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// scan reads an INI-style file and calls fn with each section header, which is
// the text between the brackets. A missing file is not an error. It yields no
// sections.
func scan(path string, fn func(section string)) error {
	f, err := os.Open(path) // #nosec G304 -- path is the user's own AWS config location
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }() // the file is read-only, so a close error is not actionable

	return scanReader(f, fn)
}

func scanReader(r io.Reader, fn func(section string)) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			fn(strings.TrimSpace(line[1 : len(line)-1]))
		}
	}
	return sc.Err()
}

func configPath() string {
	if p := os.Getenv("AWS_CONFIG_FILE"); p != "" {
		return p
	}
	return filepath.Join(homeDir(), ".aws", "config")
}

func credentialsPath() string {
	if p := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); p != "" {
		return p
	}
	return filepath.Join(homeDir(), ".aws", "credentials")
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}
