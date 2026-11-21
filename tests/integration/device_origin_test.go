package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// The person allowing a device is shown where it asked from: its address,
// the user agent it sent, and when, so a code somebody else started, and
// read out to them to allow, reads as somebody else's.
func TestLiveADevicePageSaysWhereTheDeviceAskedFrom(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	tvID, _, _ := f.register(map[string]any{
		"name": "TV", "profile": "desktop",
		"grant_types": []string{"urn:ietf:params:oauth:grant-type:device_code"},
		"scopes":      []string{"openid", "email"},
	})

	const agent = "Living-room TV/2.1 (Linux; Android 12)"
	req, _ := http.NewRequest(http.MethodPost, f.s.root+"/oauth2/device_authorization",
		strings.NewReader(url.Values{"client_id": {tvID}, "scope": {"openid email"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", agent)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var started struct {
		UserCode string `json:"user_code"`
	}
	_ = json.NewDecoder(res.Body).Decode(&started)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || started.UserCode == "" {
		t.Fatalf("starting a device = %d", res.StatusCode)
	}

	b := f.s.browser()
	_, challenge := pkce(t)
	f.signIn(b, challenge)

	var page struct {
		Device struct {
			RequestedFrom struct {
				IP        string    `json:"ip"`
				UserAgent string    `json:"user_agent"`
				At        time.Time `json:"at"`
			} `json:"requested_from"`
		} `json:"device"`
	}
	if status := b.account(http.MethodGet, "/device?user_code="+url.QueryEscape(started.UserCode), nil, &page); status != http.StatusOK {
		t.Fatalf("GET /device = %d", status)
	}
	from := page.Device.RequestedFrom
	if from.UserAgent != agent || from.IP == "" || time.Since(from.At) > time.Minute {
		t.Errorf("requested_from = %+v, want the device's agent, an address and a moment ago", from)
	}
}
