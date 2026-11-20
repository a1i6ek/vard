package model

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestAnExchangeNeverWidensWhatItsSubjectHeld(t *testing.T) {
	shop, other := uuid.New(), uuid.New()
	read := APIScope{Name: "orders:read"}
	read.ID = uuid.New()
	write := APIScope{Name: "orders:write"}
	write.ID = uuid.New()
	ada := &User{}

	tests := []struct {
		name    string
		enforce bool
		rule    []string
		held    []string
		user    *User
		roles   []UserRole
		want    []string
	}{
		{name: "what both the rule and the token name", rule: []string{"orders:read", "orders:write"}, held: []string{"openid", "orders:read"}, user: ada, want: []string{"orders:read"}},
		{name: "a scope the token holds but the rule leaves out", rule: []string{"orders:write"}, held: []string{"orders:read"}, user: ada, want: []string{}},
		{name: "an ID token, which holds no scope", rule: []string{"orders:read"}, held: []string{}, user: ada, want: []string{}},
		{name: "a service's own token, where roles do not apply", enforce: true, rule: []string{"orders:read"}, held: []string{"orders:read"}, want: []string{"orders:read"}},
		{name: "an enforcing API, and no role granting it", enforce: true, rule: []string{"orders:read"}, held: []string{"orders:read"}, user: ada, want: []string{}},
		{
			name: "an enforcing API, and a role of the subject's application granting it", enforce: true,
			rule: []string{"orders:read", "orders:write"}, held: []string{"orders:read", "orders:write"}, user: ada,
			roles: []UserRole{{ApplicationID: &shop, APIScopes: []APIScope{read}}},
			want:  []string{"orders:read"},
		},
		{
			name: "an enforcing API, and a role of another application", enforce: true,
			rule: []string{"orders:read"}, held: []string{"orders:read"}, user: ada,
			roles: []UserRole{{ApplicationID: &other, APIScopes: []APIScope{read}}},
			want:  []string{},
		},
		{
			name: "an enforcing API, and a global role", enforce: true,
			rule: []string{"orders:write"}, held: []string{"orders:write"}, user: ada,
			roles: []UserRole{{APIScopes: []APIScope{write}}},
			want:  []string{"orders:write"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := API{EnforceRoles: tt.enforce, Scopes: []APIScope{read, write}}
			rule := TokenExchangeRule{SubjectApplicationID: shop, Scopes: tt.rule}
			if got := ExchangeScopes(api, rule, tt.held, tt.user, tt.roles); !slices.Equal(got, tt.want) {
				t.Errorf("scopes = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDelegationDepthCountsNestedActors(t *testing.T) {
	tests := []struct {
		name string
		act  any
		want int
	}{
		{name: "no act claim", act: nil, want: 0},
		{name: "one actor", act: map[string]any{"sub": "a"}, want: 1},
		{name: "three nested actors", act: map[string]any{"sub": "c", "act": map[string]any{"sub": "b", "act": map[string]any{"sub": "a"}}}, want: 3},
		{name: "an act that is not an object", act: "a", want: 0},
	}
	for _, tt := range tests {
		if got := DelegationDepth(tt.act); got != tt.want {
			t.Errorf("%s: depth = %d, want %d", tt.name, got, tt.want)
		}
	}
}
