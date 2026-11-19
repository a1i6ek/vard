package authreq

import (
	"time"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/model"
)

type Grant struct {
	App      *model.Application
	User     *model.User
	Scopes   []string
	Audience string
	Nonce    string
	AuthTime time.Time
	// methods are how the person signed in (model.AMR), "" for none.
	Methods  []string
	Session  *uuid.UUID
	CodeID   *uuid.UUID
	Replaces *model.RefreshToken
	// again is the refresh token a retry within the grace is given back:
	// the one that already replaced `replaces`.
	Again string
	// dpopKey binds the tokens to a key, when the request proved one.
	DPoPKey string
	Refusal string

	// family is the refresh family the tokens belong to: the replaced
	// token's, or a new one when this grant starts one; nil for none.
	Family *uuid.UUID
}

// Confirmation is an access token's cnf: the DPoP key it is bound to.
func Confirmation(g Grant) map[string]any {
	cnf := map[string]any{}
	if g.DPoPKey != "" {
		cnf["jkt"] = g.DPoPKey
	}
	return cnf
}
