// Package respond is how every handler answers: the catalogue of problems, and
// the functions that send one or log an internal failure.
package respond

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

type Fault struct {
	Status  int
	Code    string
	Params  map[string]any
	Message string
}

func (f Fault) Error() string {
	return f.Message
}

func Fail(c *gin.Context, problem Problem, pairs ...any) {
	Write(c, problem.With(pairs...))
}

func Abort(c *gin.Context, problem Problem, pairs ...any) {
	Write(c, problem.With(pairs...))
	c.Abort()
}

// ProblemMediaType is what every answer that is not the API's success, nor an
// OAuth endpoint's own error, is sent as (RFC 9457).
const ProblemMediaType = "application/problem+json"

// problemsAt is the context key holding where problem types are named: the
// issuer, so one engine's problems never borrow another's address.
const problemsAt = "respond.problems_at"

// ProblemsAt names the problem types of the requests it sees under base, as
// base/problems/<code>. Each engine sets its own.
func ProblemsAt(base string) gin.HandlerFunc {
	base = strings.TrimRight(base, "/")
	return func(c *gin.Context) {
		c.Set(problemsAt, base)
		c.Next()
	}
}

// Write answers with a problem details object (RFC 9457): type names the
// code under the issuer, title the status, detail the English sentence, and
// code and params, as extension members, what an app says it with in the
// reader's language. A fault without a code (a model's own sentence) is
// about:blank.
func Write(c *gin.Context, fault Fault) {
	body := gin.H{
		"type":   "about:blank",
		"title":  http.StatusText(fault.Status),
		"status": fault.Status,
		"detail": fault.Message,
	}
	if fault.Code != "" {
		body["type"] = c.GetString(problemsAt) + "/problems/" + fault.Code
		body["code"] = fault.Code
	}
	if len(fault.Params) > 0 {
		body["params"] = fault.Params
	}

	c.Header("Content-Type", ProblemMediaType)
	c.JSON(fault.Status, body)
}

// Invalid passes on the sentence a model's own validation refused a value
// with, which names the field and the rule; everything else is a Problem.
func Invalid(message string) error {
	return Fault{Status: http.StatusBadRequest, Message: message}
}

var Forbidden = NotAllowed.Fault(nil)

// Failure passes a Fault through and logs anything else with `note`, answering
// with the internal problem, so database errors never reach a browser.
func Failure(c *gin.Context, log *slog.Logger, err error, note string) {
	if fault, ok := errors.AsType[Fault](err); ok {
		Write(c, fault)
		return
	}

	log.Error(note, "error", err)
	Write(c, Internal.Fault(nil))
}

// Taken answers a failed write: with `taken` when the row clashed with a
// unique value already in use (a name, an email, an identifier), and through
// Failure otherwise.
func Taken(c *gin.Context, log *slog.Logger, err error, taken Problem, note string) {
	if errors.Is(err, store.ErrDuplicate) {
		Fail(c, taken)
		return
	}

	Failure(c, log, err, note)
}

// Image sends a stored picture. Asked for by its current version, it is the
// same bytes forever, so a browser keeps it (`private` behind a session, so no
// shared cache does); asked for by any other, it is checked every time, and a
// browser that already has it is told so without the bytes. The nosniff
// header the API sends on everything keeps it an image.
func Image(c *gin.Context, contentType string, data []byte, version string, private bool) {
	cacheControl := "no-cache"
	if c.Query("v") == version {
		cacheControl = "public, max-age=31536000, immutable"
		if private {
			cacheControl = "private, max-age=31536000, immutable"
		}
	}

	etag := `"` + version + `"`
	c.Header("Cache-Control", cacheControl)
	c.Header("ETag", etag)

	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Data(http.StatusOK, contentType, data)
}
