package integration

import (
	"net/http"
	"testing"

	"github.com/a1i6ek/vard/internal/domain/model"
)

// The panel's list of applications narrows to one profile: every profile
// answers, and with only its own applications.
func TestLiveTheApplicationsListFiltersByProfile(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	registered := map[string]string{}
	for profile, body := range map[string]map[string]any{
		"web":     {"name": "Shop", "grant_types": []string{"authorization_code"}, "redirect_uris": []string{"https://shop.example.com/cb"}},
		"spa":     {"name": "Storefront", "grant_types": []string{"authorization_code"}, "redirect_uris": []string{"https://store.example.com/cb"}},
		"native":  {"name": "Phone", "grant_types": []string{"authorization_code"}, "redirect_uris": []string{"com.example.app:/cb"}},
		"desktop": {"name": "TV", "grant_types": []string{"urn:ietf:params:oauth:grant-type:device_code"}},
		"machine": {"name": "Worker", "grant_types": []string{"client_credentials"}, "token_auth_method": "private_key_jwt", "resource": "https://api.example.com"},
	} {
		body["profile"] = profile
		_, _, id := f.register(body)
		registered[profile] = id
	}

	type listed struct {
		Applications []struct {
			ID       string `json:"id"`
			ClientID string `json:"client_id"`
			Profile  string `json:"profile"`
		} `json:"applications"`
		Total int `json:"total"`
	}
	// What each profile should hold: every application, counted by profile.
	var all listed
	f.super.must(http.StatusOK, http.MethodGet, "/applications?limit=100", nil, &all)
	want := map[string]int{}
	for _, app := range all.Applications {
		want[app.Profile]++
	}

	for _, profile := range model.Profiles {
		t.Run(string(profile), func(t *testing.T) {
			var list listed
			f.super.must(http.StatusOK, http.MethodGet, "/applications?profile="+string(profile), nil, &list)

			found := false
			for _, app := range list.Applications {
				if app.Profile != string(profile) {
					t.Errorf("?profile=%s listed %s, a %s application", profile, app.ClientID, app.Profile)
				}
				found = found || app.ID == registered[string(profile)]
			}
			if !found {
				t.Errorf("?profile=%s did not list the %s application registered", profile, profile)
			}
			if list.Total != want[string(profile)] {
				t.Errorf("?profile=%s total = %d, want %d", profile, list.Total, want[string(profile)])
			}
		})
	}
}
