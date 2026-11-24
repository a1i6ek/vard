package mailsettings

import (
	"testing"

	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "MailSettings", settingsResponse{})
	spec.Match(t, "MailResponse", response{})
	spec.Match(t, "MailLanguageContent", languageContent{})
	spec.Match(t, "MailContent", contentResponse{})
}
