// Package steps takes the answers to the pages a sign-in passes once the
// user is known: the organization's terms, and what an application asks for.
package steps

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/internal/transport/http/respond"
	"github.com/a1i6ek/vard/internal/transport/http/signedin"
	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/application/oauth"
	"github.com/a1i6ek/vard/internal/application/signin"
)

type Handler struct {
	oauth *oauth.Service
	log   *slog.Logger
}

// Deps are what the handler is built from.
type Deps struct {
	OAuth *oauth.Service
	Log   *slog.Logger
}

func New(deps Deps) *Handler {
	return &Handler{oauth: deps.OAuth, log: deps.Log}
}

// Consent is what the consent page lists for a sign-in under way.
func (h *Handler) Consent(c *gin.Context) {
	page, err := h.oauth.ConsentFor(c.Request.Context(), c.Query("request"), signedin.Session(c))
	if err != nil {
		signedin.Fail(c, h.log, err, "loading a consent page failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{"consent": page})
}

// AnswerConsent takes the consent page's answer.
func (h *Handler) AnswerConsent(c *gin.Context) {
	h.answerStep(c, h.oauth.AnswerConsent, "answering a consent page failed")
}

// AnswerAgreements takes the Agreements page's answer.
func (h *Handler) AnswerAgreements(c *gin.Context) {
	h.answerStep(c, h.oauth.AnswerAgreements, "answering the agreements page failed")
}

type stepAnswer func(ctx context.Context, handle string, session *signin.Session, yes bool, client authreq.Client) (string, error)

func (h *Handler) answerStep(c *gin.Context, answer stepAnswer, note string) {
	var req stepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, respond.InvalidBody)
		return
	}
	if err := req.validate(); err != nil {
		respond.Failure(c, h.log, err, note)
		return
	}

	location, err := answer(c.Request.Context(), req.Request, signedin.Session(c), req.Yes, signedin.Client(c))
	if err != nil {
		signedin.Fail(c, h.log, err, note)
		return
	}

	c.JSON(http.StatusOK, stepResponse{RedirectTo: location})
}

// Device is what a device the person is about to allow asks for.
func (h *Handler) Device(c *gin.Context) {
	page, err := h.oauth.DeviceFor(c.Request.Context(), c.Query("user_code"), signedin.Session(c))
	if err != nil {
		signedin.Fail(c, h.log, err, "loading a device failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{"device": page})
}

// AnswerDevice allows or refuses a device for the signed-in person.
func (h *Handler) AnswerDevice(c *gin.Context) {
	var req deviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, respond.InvalidBody)
		return
	}
	if err := req.validate(); err != nil {
		respond.Failure(c, h.log, err, "validating a device answer failed")
		return
	}

	err := h.oauth.AnswerDevice(c.Request.Context(), req.UserCode, signedin.Session(c), req.Yes, req.AcceptTerms, signedin.Client(c))
	if err != nil {
		signedin.Fail(c, h.log, err, "answering a device failed")
		return
	}

	c.Status(http.StatusNoContent)
}
