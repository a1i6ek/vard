// Package keys lists the token signing keys and rotates them on demand.
package keys

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/internal/transport/http/audit"
	"github.com/a1i6ek/vard/internal/transport/http/respond"
	"github.com/a1i6ek/vard/internal/infrastructure/signing"
)

type Handler struct {
	signing *signing.Service
	audit   audit.Recorder
	log     *slog.Logger
}

// Deps are what the handler is built from.
type Deps struct {
	Signing *signing.Service
	Audit   audit.Recorder
	Log     *slog.Logger
}

func New(deps Deps) *Handler {
	return &Handler{signing: deps.Signing, audit: deps.Audit, log: deps.Log}
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
