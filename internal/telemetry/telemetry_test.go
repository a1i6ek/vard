package telemetry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestSignInOutcomesAreReadFromTheActivityLog(t *testing.T) {
	throttled := map[string]any{"reason": "throttled"}
	for _, tc := range []struct {
		name, action string
		metadata     map[string]any
		who, outcome string
		counted      bool
	}{
		{"a user signed in", "user.login", nil, "user", SignInSuccess, true},
		{"a wrong password", "user.login_failed", map[string]any{"reason": "wrong password"}, "user", SignInFailure, true},
		{"a wrong emailed code", "user.login_code_failed", nil, "user", SignInFailure, true},
		{"a wrong second factor", "user.second_factor_failed", nil, "user", SignInFailure, true},
		{"a second factor while throttled", "user.second_factor_failed", throttled, "user", SignInThrottled, true},
		{"a throttled address", "user.login_blocked", throttled, "user", SignInThrottled, true},
		{"a turned-off account", "user.login_blocked", map[string]any{"reason": "inactive"}, "user", SignInBlocked, true},
		{"an administrator signed in", "admin.login", nil, "admin", SignInSuccess, true},
		{"an administrator's wrong code", "admin.mfa_failed", nil, "admin", SignInFailure, true},
		{"a throttled administrator", "admin.login_blocked", throttled, "admin", SignInThrottled, true},
		{"a code sent is no attempt", "user.login_code_sent", nil, "", "", false},
		{"a factor added is no attempt", "admin.mfa_enabled", nil, "", "", false},
		{"another subject", "application.created", nil, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			who, outcome, counted := SignInOutcome(tc.action, tc.metadata)
			if who != tc.who || outcome != tc.outcome || counted != tc.counted {
				t.Errorf("SignInOutcome(%q) = %q, %q, %v; want %q, %q, %v", tc.action, who, outcome, counted, tc.who, tc.outcome, tc.counted)
			}
		})
	}
}

func TestNilMetricsCountNothing(t *testing.T) {
	var m *Metrics
	ctx := context.Background()
	m.SignIn(ctx, "user", SignInSuccess)
	m.Recorded(ctx, "user.login", nil)
	m.TokenIssued(ctx, "client_credentials", "machine")
	m.RefreshReuse(ctx)
	m.DPoPFailure(ctx, "invalid_dpop_proof")
	m.Logout(ctx, LogoutDelivered)
	m.Mail(ctx, MailSent)
	m.Request(ctx, "public", "GET", "/healthz", 200, time.Millisecond)
}

// Every series is served, and no label holds what a caller chose: an
// unknown method, an unmatched path, an address or an id.
func TestMetricsAreServedWithFixedLabelsOnly(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	tel, err := Start(context.Background(), "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })

	ctx := context.Background()
	m := tel.Metrics
	m.Recorded(ctx, "user.login_failed", map[string]any{"reason": "wrong password"})
	m.Recorded(ctx, "admin.login", nil)
	m.TokenIssued(ctx, "authorization_code", "spa")
	m.RefreshReuse(ctx)
	m.DPoPFailure(ctx, "use_dpop_nonce")
	m.Logout(ctx, LogoutAbandoned)
	m.Mail(ctx, MailFailed)
	m.Request(ctx, "admin", "GET", "/api/v1/admin/users/:id", 200, 5*time.Millisecond)
	m.Request(ctx, "public", "PROPFIND", "", 404, time.Millisecond)

	res := httptest.NewRecorder()
	tel.Handler().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d", res.Code)
	}
	body, _ := io.ReadAll(res.Body)
	text := string(body)

	for _, series := range []string{
		`sign_ins_total{outcome="failure",who="user"} 1`,
		`sign_ins_total{outcome="success",who="admin"} 1`,
		`tokens_issued_total{grant_type="authorization_code",profile="spa"} 1`,
		`refresh_reuse_detected_total 1`,
		`dpop_failures_total{error="use_dpop_nonce"} 1`,
		`backchannel_logouts_total{outcome="abandoned"} 1`,
		`mail_total{outcome="failed"} 1`,
		`route="/api/v1/admin/users/:id"`,
		`method="other",route="unmatched",server="public",status="4xx"`,
	} {
		if !strings.Contains(text, series) {
			t.Errorf("/metrics lacks %s", series)
		}
	}

	allowed := map[string]bool{
		"who": true, "outcome": true, "grant_type": true, "profile": true, "error": true,
		"server": true, "method": true, "route": true, "status": true, "le": true,
		"service_name": true, "service_version": true, "otel_scope_name": true,
		"otel_scope_version": true, "otel_scope_schema_url": true,
	}
	label := regexp.MustCompile(`([a-z_]+)="`)
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "{") || strings.HasPrefix(line, "go_") || strings.HasPrefix(line, "process_") || strings.HasPrefix(line, "promhttp_") {
			continue
		}
		for _, match := range label.FindAllStringSubmatch(line, -1) {
			if !allowed[match[1]] {
				t.Errorf("label %q is not one of the fixed ones: %s", match[1], line)
			}
		}
	}

	res = httptest.NewRecorder()
	tel.Handler().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusNotFound {
		t.Errorf("GET / on the metrics listener = %d, want 404: it answers /metrics alone", res.Code)
	}
}
