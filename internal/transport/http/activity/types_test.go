package activity

import (
	"testing"

	"github.com/a1i6ek/vard/internal/api/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "ActivityTarget", targetResponse{})
	spec.Match(t, "ActivityEvent", eventResponse{})
	spec.Match(t, "LogEntry", logResponse{})
	spec.Match(t, "Overview", overviewResponse{})
}
