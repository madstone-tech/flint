package cli

import (
	"errors"
	"fmt"
)

// Exit codes. These are a stable contract so scripts can branch on failures.
const (
	exitOK          = 0   // success
	exitGeneric     = 1   // unspecified error
	exitUsage       = 2   // bad invocation (flags/args)
	exitCredentials = 3   // credentials/region could not be resolved
	exitRejected    = 4   // token structurally bad, expired, or rejected by Bedrock
	exitAborted     = 130 // user cancelled (SIGINT convention)
)

// exitError carries an exit code and an optional remediation hint with the
// underlying error. The top-level handler uses the code. flint prints the hint
// to stderr to tell the user what to do next.
type exitError struct {
	code int
	hint string
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// withExit wraps err with an exit code and a remediation hint.
func withExit(code int, hint string, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, hint: hint, err: err}
}

// credentialError flags a credentials/region resolution failure with guidance.
func credentialError(err error, profile string) error {
	var hint string
	if profile != "" {
		hint = fmt.Sprintf("try `aws sso login --profile %s`, or verify the profile in ~/.aws/config", profile)
	} else {
		hint = "set AWS_PROFILE, pass a profile, or run `aws sso login`"
	}
	return withExit(exitCredentials, hint, err)
}

// codeFor extracts the exit code from an error, defaulting to exitGeneric.
func codeFor(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return exitGeneric
}

// hintFor extracts the remediation hint from an error, if any.
func hintFor(err error) string {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.hint
	}
	return ""
}
