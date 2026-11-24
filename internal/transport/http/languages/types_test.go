package languages

import (
	"testing"

	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "Language", languageResponse{})
	spec.Match(t, "LanguageList", listResponse{})
	spec.Match(t, "Translation", translationResponse{})
}
