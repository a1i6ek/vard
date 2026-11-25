package activity

import (
	"net/http"
	"regexp"

	"github.com/a1i6ek/vard/internal/infrastructure/store"
	"github.com/a1i6ek/vard/internal/transport/http/respond"
)

var filterInvalid = respond.Define(http.StatusBadRequest, "logs_filter_invalid", respond.Admin)

// actionPattern is "<resource>.<action>"; names are checked and lists capped
// before reaching a query.
var actionPattern = regexp.MustCompile(`^[a-z_]{1,64}\.[a-z_]{1,64}$`)

// targetTypePattern is a record's kind as audit.Record names it ("user",
// "sso_connection").
var targetTypePattern = regexp.MustCompile(`^[a-z_]{1,64}$`)

const (
	maxActions     = 200
	maxTargetTypes = 20
	maxIPLength    = 45

	// countLimit is as far as a page's total is counted; past it the panel
	// says "more than".
	countLimit = 1000
)

var actorTypes = map[string]bool{store.ActorAdmin: true, store.ActorUser: true, store.ActorSystem: true}
