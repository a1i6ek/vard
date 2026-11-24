// Package otp manages the settings for one-time codes sent by email.
package otp

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/internal/transport/http/audit"
	"github.com/a1i6ek/vard/internal/transport/http/respond"
	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

type Handler struct {
	store *store.Store
	audit audit.Recorder
	log   *slog.Logger
}

// Deps are what the handler is built from.
type Deps struct {
	Store *store.Store
	Audit audit.Recorder
	Log   *slog.Logger
}

func New(deps Deps) *Handler {
	return &Handler{store: deps.Store, audit: deps.Audit, log: deps.Log}
}

// Get returns the settings and the login flows that use emailed codes.
func (h *Handler) Get(c *gin.Context) {
	ctx := c.Request.Context()

	settings, err := h.store.OTPSettings(ctx)
	if err != nil {
		respond.Failure(c, h.log, err, "loading the one-time code settings failed")
		return
	}

	flows, err := h.store.LoginFlows(ctx)
	if err != nil {
		respond.Failure(c, h.log, err, "listing login flows failed")
		return
	}

	c.JSON(http.StatusOK, newResponse(*settings, flows))
}

// Update changes the settings. What a request leaves out is left as it is.
func (h *Handler) Update(c *gin.Context) {
	ctx := c.Request.Context()

	var req settingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, respond.InvalidBody)
		return
	}

	var changed []string
	settings, err := h.store.UpdateOTPSettings(ctx, func(settings *model.OTPSettings) (bool, error) {
		var err error
		changed, err = req.applyTo(settings)
		return len(changed) > 0, err
	})
	if err != nil {
		respond.Failure(c, h.log, err, "updating the one-time code settings failed")
		return
	}

	flows, err := h.store.LoginFlows(ctx)
	if err != nil {
		respond.Failure(c, h.log, err, "listing login flows failed")
		return
	}

	if len(changed) == 0 {
		c.JSON(http.StatusOK, newResponse(*settings, flows))
		return
	}

	// A code that is shorter or lasts longer is a change to how hard this
	// server is to get into, so the log says which way it moved.
	h.audit.RecordWith(c, "otp.settings_updated", targetType, targetID, map[string]any{
		"fields":           changed,
		"code_length":      settings.CodeLength,
		"lifetime_minutes": settings.LifetimeMinutes,
		"max_attempts":     settings.MaxAttempts,
	})

	c.JSON(http.StatusOK, newResponse(*settings, flows))
}
