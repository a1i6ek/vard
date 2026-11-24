package activity

import (
	"net/http"
	"regexp"

	"github.com/a1i6ek/vard/internal/api/respond"
)

var filterInvalid = respond.Define(http.StatusBadRequest, "logs_filter_invalid", respond.Admin)

// actionPattern is "<resource>.<action>"; names are checked and lists capped
// before reaching a query.
var actionPattern = regexp.MustCompile(`^[a-z_]{1,64}\.[a-z_]{1,64}$`)

const maxActions = 100
