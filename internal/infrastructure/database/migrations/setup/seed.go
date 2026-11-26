package setup

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/a1i6ek/vard/internal/domain/model"
)

// seed writes the rows an installation needs to start: the admin roles, the
// organisation, the default login flow and English. Columns are named, so a
// later model change cannot change it. There is no administrator: the first
// is made at setup, so no default password exists anywhere.
func seed(db *gorm.DB) error {
	now := time.Now()
	row := func(what, statement string, values ...any) error {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if err := db.Exec(statement, append([]any{id, now, now}, values...)...).Error; err != nil {
			return fmt.Errorf("create %s: %w", what, err)
		}
		return nil
	}

	for _, role := range startingRoles {
		var permissions any
		if role.Permissions != nil {
			encoded, err := json.Marshal(role.Permissions)
			if err != nil {
				return err
			}
			permissions = string(encoded)
		}
		err := row("the admin role "+role.Name, `INSERT INTO admin_roles
			(id, created_at, updated_at, name, description, permissions) VALUES (?, ?, ?, ?, ?, ?)`,
			role.Name, role.Description, permissions)
		if err != nil {
			return err
		}
	}

	organization := model.DefaultOrganization()
	err := row("the organization", `INSERT INTO organizations
		(id, created_at, updated_at, name, slug, timezone) VALUES (?, ?, ?, ?, ?, ?)`,
		organization.Name, organization.Slug, organization.Timezone)
	if err != nil {
		return err
	}

	flow := model.DefaultLoginFlow()
	steps, err := json.Marshal(flow.Steps)
	if err != nil {
		return err
	}
	err = row("the default login flow", `INSERT INTO login_flows
		(id, created_at, updated_at, name, slug, description, is_default, is_enabled, steps,
		 allow_sign_in, allow_registration, allow_password_reset, allow_remember_me,
		 verify_email_on_register, require_verified_email, allow_email_change, session_lifetime_hours,
		 require_second_factor)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		flow.Name, flow.Slug, flow.Description, flow.IsDefault, flow.IsEnabled, string(steps),
		flow.AllowSignIn, flow.AllowRegistration, flow.AllowPasswordReset, flow.AllowRememberMe,
		flow.VerifyEmailOnRegister, flow.RequireVerifiedEmail, flow.AllowEmailChange, flow.SessionLifetimeHours,
		flow.RequireSecondFactor)
	if err != nil {
		return err
	}

	language := model.DefaultLanguage()
	return row("the default language", `INSERT INTO languages
		(id, created_at, updated_at, code, name, native_name, is_enabled, is_default, position)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		language.Code, language.Name, language.NativeName, language.IsEnabled, language.IsDefault, language.Position)
}

// startingRoles are the admin roles a panel is given, and what each grants.
//
// super_admin is built in: it grants every permission by name, so it lists
// none, and it is the only role that can manage administrators. A super admin
// can change or remove any of the others. app_manager is meant to be held for
// one application: every one of its permissions can be scoped.
var startingRoles = []model.AdminRole{
	{
		Name:        model.RoleSuperAdmin,
		Description: "Everything, including managing administrators",
	},
	{
		Name:        "admin",
		Description: "Everything but managing administrators",
		Permissions: model.AdminPermissionNames(),
	},
	{
		Name:        "moderator",
		Description: "Manage users, application roles and who holds them, and read the activity log",
		Permissions: []string{
			model.PermActivityRead, model.PermUsersRead, model.PermUsersWrite, model.PermAPIsRead,
			model.PermApplicationsRead, model.PermUserRolesWrite, model.PermRoleAssignmentsWrite,
		},
	},
	{
		Name:        "app_manager",
		Description: "Look after an application: its settings, its roles, and who holds them",
		Permissions: []string{
			model.PermApplicationsRead, model.PermApplicationsWrite,
			model.PermUserRolesWrite, model.PermRoleAssignmentsWrite,
		},
	},
	{
		Name:        "support",
		Description: "Look users up, fix their records and give them application roles",
		Permissions: []string{
			model.PermUsersRead, model.PermUsersWrite, model.PermApplicationsRead, model.PermRoleAssignmentsWrite,
		},
	},
	{
		Name:        "auditor",
		Description: "Read only, including the activity log",
		Permissions: []string{
			model.PermActivityRead, model.PermUsersRead, model.PermAPIsRead, model.PermApplicationsRead,
		},
	},
}
