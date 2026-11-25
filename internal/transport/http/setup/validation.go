package setup

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/transport/http/respond"
	"github.com/a1i6ek/vard/internal/transport/http/validate"
)

var (
	alreadySetUp = respond.Define(http.StatusConflict, "setup_done", respond.Admin)
)

func (r *setupRequest) validate() error {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.FirstName = strings.TrimSpace(r.FirstName)
	r.LastName = strings.TrimSpace(r.LastName)

	if err := validate.Struct(r); err != nil {
		return err
	}
	if utf8.RuneCountInString(r.Password) < model.MinAdminPasswordLength {
		return respond.AdminPasswordTooShort.With("min", model.MinAdminPasswordLength)
	}

	return nil
}
