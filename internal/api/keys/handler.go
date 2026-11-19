// Package keys lists the token signing keys and rotates them on demand.
package keys

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/internal/api/audit"
	"github.com/a1i6ek/vard/internal/api/respond"
	"github.com/a1i6ek/vard/internal/api/services"
	"github.com/a1i6ek/vard/internal/signing"
)

type Handler struct {
	signing *signing.Service
	audit   audit.Recorder
	log     *slog.Logger
}

func New(provider *services.Provider, recorder audit.Recorder, log *slog.Logger) *Handler {
	return &Handler{signing: provider.Signing, audit: recorder, log: log}
}

// List describes every published key: the one signing, the next one waiting,
// and the retired ones still published.
func (h *Handler) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"keys": h.signing.SigningKeys()})
}

// Rotate makes new keys now.
func (h *Handler) Rotate(c *gin.Context) {
	var req rotateRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respond.Fail(c, respond.InvalidBody)
			return
		}
	}

	if err := h.signing.RotateKeys(c.Request.Context(), req.Immediate, req.RevokeOld); err != nil {
		respond.Failure(c, h.log, err, "rotating the signing keys failed")
		return
	}

	h.audit.RecordWith(c, "signing_keys.rotated", "", "", map[string]any{
		"immediate":  req.Immediate || req.RevokeOld,
		"revoke_old": req.RevokeOld,
	})

	c.JSON(http.StatusOK, gin.H{"keys": h.signing.SigningKeys()})
}
