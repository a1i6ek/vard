package users

import (
	"testing"

	"github.com/a1i6ek/vard/internal/api/spec"
	"github.com/a1i6ek/vard/internal/model"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "UserRecord", model.User{})
	spec.Match(t, "UserPage", pageResponse{})
	spec.Match(t, "RoleMapping", mappedRole{})
	spec.Match(t, "UserRoles", rolesResponse{})
	spec.Match(t, "HeldRoles", heldRoles{})
	spec.Match(t, "ApplicationRoles", applicationRoles{})
}
