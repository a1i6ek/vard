package authreq

import (
	"fmt"
)

var ErrSSORequired = NewProblem("sso_required")

// ErrBrowserRequired refuses an answer to the Agreements or Consent page that
// came with an application's access token: only the person, in their own
// browser, may accept terms or allow an application.
var ErrBrowserRequired = NewProblem("browser_session_required")

type SSORefusal struct {
	Shown *Problem
	Step  string
	Err   error
}

func Refused(shown *Problem, format string, args ...any) error {
	return &SSORefusal{Shown: shown, Step: "claims", Err: fmt.Errorf(format, args...)}
}

func (f *SSORefusal) Error() string { return f.Step + ": " + f.Err.Error() }

func (f *SSORefusal) Unwrap() error { return f.Err }
