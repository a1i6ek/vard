package admins

import (
	"testing"

	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "AdminRecord", adminResponse{})
	spec.Match(t, "AdminAssignment", assignmentResponse{})
	spec.Match(t, "AdminPage", pageResponse{})
	spec.Match(t, "Reference", reference{})
}
