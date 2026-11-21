package integration

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// A mobile app's builds are published from the sign-in origin, so the app
// may use the passkeys made there: iOS app IDs in apple-app-site-association
// (webcredentials), Android packages and their certificates in
// assetlinks.json (get_login_creds). With none, there is no file; an app
// turned off, or any other kind of application, publishes nothing.
func TestLiveMobileAppsMayUseTheSignInPasskeys(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	fetch := func(path string, out any) int {
		t.Helper()
		res, err := http.Get(f.s.root + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if out != nil && res.StatusCode == http.StatusOK {
			if err := json.NewDecoder(res.Body).Decode(out); err != nil {
				t.Fatal(err)
			}
		}
		return res.StatusCode
	}

	for _, path := range []string{"/.well-known/apple-app-site-association", "/.well-known/assetlinks.json"} {
		if status := fetch(path, nil); status != http.StatusNotFound {
			t.Errorf("%s with no mobile app = %d, want 404", path, status)
		}
	}

	fingerprint := strings.Repeat("14:", 31) + "14"
	_, _, appID := f.register(map[string]any{
		"name": "Shop for iOS and Android", "profile": "native",
		"grant_types": []string{"authorization_code"}, "redirect_uris": []string{"com.example.shop:/callback"},
		"ios_app_ids":  []string{"ABCDE12345.com.example.shop"},
		"android_apps": []map[string]any{{"package_name": "com.example.shop", "sha256_cert_fingerprints": []string{strings.ToLower(fingerprint)}}},
	})

	var apple struct {
		WebCredentials struct {
			Apps []string `json:"apps"`
		} `json:"webcredentials"`
	}
	if status := fetch("/.well-known/apple-app-site-association", &apple); status != http.StatusOK || !slices.Equal(apple.WebCredentials.Apps, []string{"ABCDE12345.com.example.shop"}) {
		t.Errorf("apple-app-site-association = %d %+v", status, apple)
	}

	var links []struct {
		Relation []string `json:"relation"`
		Target   struct {
			Namespace    string   `json:"namespace"`
			PackageName  string   `json:"package_name"`
			Fingerprints []string `json:"sha256_cert_fingerprints"`
		} `json:"target"`
	}
	if status := fetch("/.well-known/assetlinks.json", &links); status != http.StatusOK || len(links) != 1 ||
		links[0].Relation[0] != "delegate_permission/common.get_login_creds" || links[0].Target.PackageName != "com.example.shop" ||
		!slices.Equal(links[0].Target.Fingerprints, []string{fingerprint}) {
		t.Errorf("assetlinks.json = %d %+v", status, links)
	}

	// A web application cannot publish builds, whatever it sends.
	var web struct {
		Application struct {
			IOSAppIDs []string `json:"ios_app_ids"`
		} `json:"application"`
	}
	f.super.must(http.StatusCreated, http.MethodPost, "/applications", map[string]any{
		"name": "Back office", "profile": "web", "grant_types": []string{"authorization_code"},
		"redirect_uris": []string{shopRedirect}, "ios_app_ids": []string{"ZZZZZ99999.com.example.web"},
	}, &web)
	if len(web.Application.IOSAppIDs) != 0 {
		t.Errorf("a web application kept iOS app IDs %v", web.Application.IOSAppIDs)
	}

	var got struct {
		Application map[string]any `json:"application"`
	}
	f.super.must(http.StatusOK, http.MethodGet, "/applications/"+appID, nil, &got)
	got.Application["is_enabled"] = false
	f.super.must(http.StatusOK, http.MethodPatch, "/applications/"+appID, got.Application, nil)
	if status := fetch("/.well-known/apple-app-site-association", nil); status != http.StatusNotFound {
		t.Errorf("with the mobile app turned off = %d, want 404", status)
	}
}
