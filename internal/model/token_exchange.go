package model

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// RFC 8693's grant type, and the token types it reads and issues.
const (
	GrantTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"
	TokenTypeAccess    = "urn:ietf:params:oauth:token-type:access_token"
	TokenTypeID        = "urn:ietf:params:oauth:token-type:id_token"
)

// MaxDelegationDepth is how many actors an exchanged token's act claim may
// nest (RFC 8693 §4.1): a service calling a service calling a service, and
// no deeper, so a chain of delegation stays one an administrator can follow.
const MaxDelegationDepth = 3

// TokenExchangeRule lets one application exchange (RFC 8693) a token issued
// to another for a token to this API, acting for the token's subject, with
// at most the scopes listed. Delegation is never inferred, from being
// allowed to request the API's tokens or anything else: without a rule
// naming all three, an exchange is refused. Only a confidential application
// may act.
type TokenExchangeRule struct {
	APIID                uuid.UUID    `gorm:"type:uuid;primaryKey" json:"api_id"`
	API                  *API         `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	ApplicationID        uuid.UUID    `gorm:"type:uuid;primaryKey;index:idx_token_exchange_rules_application" json:"application_id"`
	Application          *Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	SubjectApplicationID uuid.UUID    `gorm:"type:uuid;primaryKey;index:idx_token_exchange_rules_subject_application" json:"subject_application_id"`
	SubjectApplication   *Application `gorm:"foreignKey:SubjectApplicationID;constraint:OnDelete:CASCADE" json:"-"`
	Scopes               []string     `gorm:"type:jsonb;serializer:json;not null" json:"scopes"`
	CreatedAt            time.Time    `gorm:"not null" json:"created_at"`
}

func (TokenExchangeRule) TableName() string {
	return "token_exchange_rules"
}

// ExchangeScopes is the most an exchanged token may carry: the API's scopes
// the rule lists and the subject token already holds, so an exchange never
// widens what the subject could do. Where the API enforces roles, a user's
// token keeps only what a role of theirs grants: a global one, or one of
// the application the subject token was issued to.
func ExchangeScopes(api API, rule TokenExchangeRule, held []string, user *User, roles []UserRole) []string {
	granting := map[uuid.UUID]bool{}
	for _, role := range roles {
		if !role.Global() && !role.In(&rule.SubjectApplicationID) {
			continue
		}
		for _, scope := range role.APIScopes {
			granting[scope.ID] = true
		}
	}

	out := []string{}
	for _, scope := range api.Scopes {
		switch {
		case !slices.Contains(rule.Scopes, scope.Name), !slices.Contains(held, scope.Name):
		case user != nil && api.EnforceRoles && !granting[scope.ID]:
		default:
			out = append(out, scope.Name)
		}
	}
	return out
}

// DelegationDepth is how many actors an act claim nests (RFC 8693 §4.1),
// 0 for none.
func DelegationDepth(act any) int {
	depth := 0
	for {
		claim, ok := act.(map[string]any)
		if !ok {
			return depth
		}
		depth++
		act = claim["act"]
	}
}
