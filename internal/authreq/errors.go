package authreq

import (
	"net/http"
)

const (
	ErrInvalidRequest          = "invalid_request"
	ErrInvalidClient           = "invalid_client"
	ErrInvalidGrant            = "invalid_grant"
	ErrUnauthorizedClient      = "unauthorized_client"
	ErrUnsupportedGrantType    = "unsupported_grant_type"
	ErrUnsupportedResponseType = "unsupported_response_type"
	ErrInvalidScope            = "invalid_scope"
	ErrAccessDenied            = "access_denied"
	ErrServerError             = "server_error"
	ErrLoginRequired           = "login_required"
	ErrInvalidToken            = "invalid_token"
	ErrInsufficientScope       = "insufficient_scope"

	// OpenID Connect Core §3.1.2.6: parameters the provider does not take.
	ErrRequestNotSupported      = "request_not_supported"
	ErrRegistrationNotSupported = "registration_not_supported"
)

type Error struct {
	Code        string
	Description string
	Status      int
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Description
}

func OAuthError(code, description string) *Error {
	status := http.StatusBadRequest
	switch code {
	case ErrInvalidClient, ErrInvalidToken:
		status = http.StatusUnauthorized
	case ErrInsufficientScope:
		status = http.StatusForbidden
	}

	return &Error{Code: code, Description: description, Status: status}
}

// Problem is why a user's sign-in or change was refused. It carries only its
// code: what the user reads is `error.<code>` in i18n/id, and the HTTP status
// is chosen in api/account (`provided`), since this package knows
// nothing of HTTP.
type Problem struct {
	Code string
}

func (p *Problem) Error() string {
	return p.Code
}

var problems []*Problem

func NewProblem(code string) *Problem {
	p := &Problem{Code: code}
	problems = append(problems, p)

	return p
}

func Problems() []*Problem {
	return problems
}

// The errors of signing a user in. They are what the sign-in pages show, so
// they say what went wrong without saying which accounts exist.
var (
	ErrInvalidCredentials    = NewProblem("invalid_credentials")
	ErrRequestExpired        = NewProblem("request_expired")
	ErrRegistrationClosed    = NewProblem("registration_closed")
	ErrEmailTaken            = NewProblem("email_taken")
	ErrResetInvalid          = NewProblem("reset_invalid")
	ErrTermsRequired         = NewProblem("terms_required")
	ErrDetailsRequired       = NewProblem("details_required")
	ErrPasswordTooLong       = NewProblem("password_too_long")
	ErrPasswordNotOffered    = NewProblem("password_not_offered")
	ErrEmailNotVerified      = NewProblem("email_not_verified")
	ErrVerificationInvalid   = NewProblem("verification_invalid")
	ErrSignInClosed          = NewProblem("sign_in_closed")
	ErrEmailChangeNotOffered = NewProblem("email_change_not_offered")

	ErrCodeInvalid      = NewProblem("code_invalid")
	ErrCodeExpired      = NewProblem("code_expired")
	ErrCodeAttemptsUsed = NewProblem("code_attempts_used")
	ErrCodeTooSoon      = NewProblem("code_too_soon")

	ErrSocialUnknown            = NewProblem("social_unknown")
	ErrSocialExpired            = NewProblem("social_expired")
	ErrSocialUpstream           = NewProblem("social_upstream")
	ErrSocialNoEmail            = NewProblem("social_no_email")
	ErrSocialLinkRefused        = NewProblem("social_link_refused")
	ErrSocialEmailUnverified    = NewProblem("social_email_unverified")
	ErrSocialRegistrationClosed = NewProblem("social_registration_closed")
	ErrSocialSSORequired        = NewProblem("social_sso_required")
	ErrSocialBlocked            = NewProblem("social_blocked")
	ErrSocialNotOffered         = NewProblem("social_not_offered")
)
