package oauth

import (
	"testing"

	"github.com/a1i6ek/vard/internal/domain/model"
)

// A registering client never names its profile: it is read from what the
// client says it does, and a combination no profile allows is refused
// rather than turned into one that is.
func TestARegisteredClientGetsItsProfile(t *testing.T) {
	code := []string{model.GrantAuthorizationCode}
	service := []string{model.GrantClientCredentials}

	tests := []struct {
		name   string
		meta   ClientMetadata
		grants []string
		want   model.Profile // "" when refused
	}{
		{"a confidential web client", ClientMetadata{}, code, model.ProfileWeb},
		{"a web client signing assertions", ClientMetadata{TokenEndpointAuthMethod: "private_key_jwt"}, code, model.ProfileWeb},
		{"a browser client with no credential", ClientMetadata{TokenEndpointAuthMethod: "none"}, code, model.ProfileSPA},
		{"a native client sent back to its scheme", ClientMetadata{ApplicationType: "native", RedirectURIs: []string{"com.example.app:/cb"}}, code, model.ProfileNative},
		{"a native client sent back to loopback", ClientMetadata{ApplicationType: "native", RedirectURIs: []string{"http://127.0.0.1/cb"}}, code, model.ProfileDesktop},
		{"a native client with the device grant alone", ClientMetadata{ApplicationType: "native"}, []string{model.GrantDeviceCode}, model.ProfileDesktop},
		{"a service signing assertions", ClientMetadata{TokenEndpointAuthMethod: "private_key_jwt"}, service, model.ProfileMachine},
		{"a service with a secret", ClientMetadata{TokenEndpointAuthMethod: "client_secret_basic"}, service, ""},
		{"a service that says nothing of how it authenticates", ClientMetadata{}, service, ""},
		{"a native service", ClientMetadata{ApplicationType: "native", TokenEndpointAuthMethod: "private_key_jwt"}, service, ""},
		{"a native client with a secret", ClientMetadata{ApplicationType: "native", TokenEndpointAuthMethod: "client_secret_post"}, code, ""},
		{"an application type nobody has", ClientMetadata{ApplicationType: "mainframe"}, code, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := profileOf(tt.meta, tt.grants)
			switch {
			case tt.want == "" && err == nil:
				t.Errorf("profileOf = %s, want it refused", got)
			case tt.want != "" && (err != nil || got != tt.want):
				t.Errorf("profileOf = %s, %v, want %s", got, err, tt.want)
			}
		})
	}
}
