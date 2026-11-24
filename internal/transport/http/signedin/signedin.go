// Package signedin is what the packages answering a person about their own
// account share: the session the request is signed in with, who is calling
// as the provider sees them, and how the provider's refusals are answered.
package signedin

import (
	"errors"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/internal/api/respond"
	"github.com/a1i6ek/vard/internal/api/session"
	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/federation"
	"github.com/a1i6ek/vard/internal/model"
	"github.com/a1i6ek/vard/internal/oauth"
	"github.com/a1i6ek/vard/internal/signin"
)

const sessionKey = "user_session"

var passwordTooShort = respond.Define(http.StatusBadRequest, "password_too_short", respond.Public)

// CheckPassword enforces length only: what else a password must be is a rule
// that pushes people to worse ones.
func CheckPassword(password string) error {
	switch {
	case utf8.RuneCountInString(password) < model.MinUserPasswordLength:
		return passwordTooShort.With("min", model.MinUserPasswordLength)
	case utf8.RuneCountInString(password) > model.MaxPasswordLength:
		return Problem(authreq.ErrPasswordTooLong).Fault(nil)
	}

	return nil
}

// Set keeps the request's signed-in session for the handlers after it.
func Set(c *gin.Context, session *signin.Session) {
	c.Set(sessionKey, session)
}

// Session is the request's signed-in session, as a route guard set it.
func Session(c *gin.Context) *signin.Session {
	value, _ := c.Get(sessionKey)
	current, _ := value.(*signin.Session)
	return current
}

// Client is who is calling, as the provider records it.
func Client(c *gin.Context) authreq.Client {
	return authreq.Client{IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Language: session.Language(c), KnownBrowser: session.KnownBrowser(c)}
}

// Problem is the API problem a provider problem is answered with.
func Problem(refused *authreq.Problem) respond.Problem {
	return problems[refused.Code]
}

// Fail answers an error from the provider: a refusal as its problem, single
// sign-on required with the connection to use, anything else as a failure.
func Fail(c *gin.Context, log *slog.Logger, err error, note string) {
	if sso, ok := errors.AsType[*signin.SSORequired](err); ok {
		respond.Fail(c, problems[authreq.ErrSSORequired.Code], "slug", sso.Slug, "name", sso.Name)
		return
	}

	if refused, ok := errors.AsType[*authreq.Problem](err); ok {
		if problem, ok := problems[refused.Code]; ok {
			respond.Write(c, problem.Fault(nil))
			return
		}
	}

	respond.Failure(c, log, err, note)
}

// problems maps every provider problem (authreq.Problems) to a public API problem
// by code; only the status differs, so new problems need just a catalog
// sentence. Defined once here, since a code can only be defined once.
var problems = func() map[string]respond.Problem {
	statuses := map[string]int{
		authreq.ErrInvalidCredentials.Code:          http.StatusUnauthorized,
		authreq.ErrRequestExpired.Code:              http.StatusGone,
		authreq.ErrResetInvalid.Code:                http.StatusGone,
		authreq.ErrRegistrationClosed.Code:          http.StatusForbidden,
		authreq.ErrEmailTaken.Code:                  http.StatusConflict,
		signin.ErrNotYours.Code:                     http.StatusNotFound,
		authreq.ErrSSORequired.Code:                 http.StatusForbidden,
		federation.ErrSSOUnknown.Code:               http.StatusNotFound,
		authreq.ErrPasswordNotOffered.Code:          http.StatusForbidden,
		authreq.ErrEmailNotVerified.Code:            http.StatusForbidden,
		authreq.ErrVerificationInvalid.Code:         http.StatusGone,
		signin.ErrVerificationPasswordRequired.Code: http.StatusForbidden,
		signin.ErrVerificationSignInRequired.Code:   http.StatusForbidden,
		authreq.ErrCodeExpired.Code:                 http.StatusGone,
		authreq.ErrCodeAttemptsUsed.Code:            http.StatusGone,
		authreq.ErrCodeTooSoon.Code:                 http.StatusTooManyRequests,
		authreq.ErrSignInClosed.Code:                http.StatusForbidden,
		authreq.ErrEmailChangeNotOffered.Code:       http.StatusForbidden,
		authreq.ErrBrowserRequired.Code:             http.StatusForbidden,
		signin.ErrSecondFactorExpired.Code:          http.StatusGone,
		signin.ErrTwoStepNotOffered.Code:            http.StatusForbidden,
		signin.ErrAuthenticatorExists.Code:          http.StatusConflict,
		signin.ErrPasskeyLimit.Code:                 http.StatusConflict,
		oauth.ErrDeviceCodeInvalid.Code:             http.StatusNotFound,
		oauth.ErrDeviceRefused.Code:                 http.StatusForbidden,
		signin.ErrSignInAgain.Code:                  http.StatusForbidden,
		signin.ErrSecondFactorRequired.Code:         http.StatusForbidden,
		oauth.ErrStepUpRequired.Code:                http.StatusForbidden,
		oauth.ErrAgreementsRequired.Code:            http.StatusForbidden,
	}

	out := map[string]respond.Problem{}
	for _, refused := range authreq.Problems() {
		status, ok := statuses[refused.Code]
		if !ok {
			status = http.StatusBadRequest
		}

		out[refused.Code] = respond.Define(status, refused.Code, respond.Public)
	}

	return out
}()
