// Package telemetry is what the server says about itself: counts and
// timings in Prometheus's format on a listener of their own, and traces and
// metrics sent over OTLP only when OTEL_EXPORTER_OTLP_ENDPOINT (or its
// _TRACES_ / _METRICS_ variant) says where.
//
// No label holds personal data or anything a caller chooses: no address,
// user, client id or raw path, only route templates and values from fixed
// sets, so a stranger cannot grow a series or read anyone from it.
package telemetry

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/a1i6ek/vard/internal/brand"
)

// Telemetry is the running exporters: the Prometheus handler for /metrics,
// and what has to be flushed on the way out.
type Telemetry struct {
	Metrics *Metrics
	Tracer  trace.Tracer

	registry *prometheus.Registry
	stop     []func(context.Context) error
}

// Start makes the meter and, when the environment names an OTLP endpoint,
// the exporters that send to it. db, when given, is watched for its pool.
func Start(ctx context.Context, version string, db *sql.DB) (*Telemetry, error) {
	t := &Telemetry{registry: prometheus.NewRegistry()}
	t.registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	res := resource.NewSchemaless(
		attribute.String("service.name", brand.Slug),
		attribute.String("service.version", version),
	)

	scrape, err := otelprometheus.New(otelprometheus.WithRegisterer(t.registry), otelprometheus.WithoutScopeInfo())
	if err != nil {
		return nil, err
	}
	readers := []sdkmetric.Option{sdkmetric.WithResource(res), sdkmetric.WithReader(scrape)}
	if configured("METRICS") {
		push, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, err
		}
		readers = append(readers, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(push)))
	}
	meters := sdkmetric.NewMeterProvider(readers...)
	t.stop = append(t.stop, meters.Shutdown)

	t.Tracer = noop.NewTracerProvider().Tracer(brand.Slug)
	if configured("TRACES") {
		spans, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, err
		}
		tracers := sdktrace.NewTracerProvider(sdktrace.WithBatcher(spans), sdktrace.WithResource(res))
		otel.SetTracerProvider(tracers)
		otel.SetTextMapPropagator(propagation.TraceContext{})
		t.Tracer = tracers.Tracer(brand.Slug)
		t.stop = append(t.stop, tracers.Shutdown)
	}

	t.Metrics, err = newMetrics(meters.Meter(brand.Slug))
	if err != nil {
		return nil, err
	}
	if db != nil {
		if err := t.Metrics.watchPool(db); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// configured says whether the standard OpenTelemetry variables name an
// endpoint for this signal: nothing is sent anywhere otherwise.
func configured(signal string) bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_"+signal+"_ENDPOINT") != ""
}

// Handler answers GET /metrics in Prometheus's text format.
func (t *Telemetry) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(t.registry, promhttp.HandlerOpts{}))
	return mux
}

// Shutdown sends what is waiting and stops the exporters.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	for _, stop := range t.stop {
		errs = append(errs, stop(ctx))
	}
	return errors.Join(errs...)
}

// Metrics are the server's counts. Its methods do nothing on a nil
// *Metrics, so a test or a tool that never starts telemetry needs none.
type Metrics struct {
	signIns       metric.Int64Counter
	tokens        metric.Int64Counter
	refreshReuse  metric.Int64Counter
	dpopFailures  metric.Int64Counter
	logouts       metric.Int64Counter
	mail          metric.Int64Counter
	requestTiming metric.Float64Histogram
	meter         metric.Meter
}

func newMetrics(meter metric.Meter) (*Metrics, error) {
	m := &Metrics{meter: meter}
	var errs [7]error
	m.signIns, errs[0] = meter.Int64Counter(name("sign_ins"),
		metric.WithDescription("Sign-in attempts by who (user, admin) and outcome (success, failure, throttled, blocked)."))
	m.tokens, errs[1] = meter.Int64Counter(name("tokens_issued"),
		metric.WithDescription("Token responses by grant type and the application's profile."))
	m.refreshReuse, errs[2] = meter.Int64Counter(name("refresh_reuse_detected"),
		metric.WithDescription("Replaced refresh tokens presented again, each of which revoked its family."))
	m.dpopFailures, errs[3] = meter.Int64Counter(name("dpop_failures"),
		metric.WithDescription("DPoP proofs refused, by error: invalid_dpop_proof, or use_dpop_nonce asking for a retry."))
	m.logouts, errs[4] = meter.Int64Counter(name("backchannel_logouts"),
		metric.WithDescription("Back-channel logout attempts by outcome: delivered, retrying, abandoned."))
	m.mail, errs[5] = meter.Int64Counter(name("mail"),
		metric.WithDescription("Messages by outcome: sent, failed, or off (sending is turned off)."))
	m.requestTiming, errs[6] = meter.Float64Histogram(name("http.server.request.duration"),
		metric.WithUnit("s"),
		metric.WithDescription("Time to answer a request, by listener, method, route template and status class."),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10))
	return m, errors.Join(errs[:]...)
}

// watchPool reports the database pool when the meter is read.
func (m *Metrics) watchPool(db *sql.DB) error {
	open, err1 := m.meter.Int64ObservableGauge(name("db.pool.open"), metric.WithDescription("Connections open."))
	inUse, err2 := m.meter.Int64ObservableGauge(name("db.pool.in_use"), metric.WithDescription("Connections in use."))
	waits, err3 := m.meter.Int64ObservableCounter(name("db.pool.waits"), metric.WithDescription("Times a query waited for a connection."))
	waited, err4 := m.meter.Float64ObservableCounter(name("db.pool.wait_duration"), metric.WithUnit("s"),
		metric.WithDescription("Time spent waiting for a connection."))
	if err := errors.Join(err1, err2, err3, err4); err != nil {
		return err
	}
	_, err := m.meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		stats := db.Stats()
		o.ObserveInt64(open, int64(stats.OpenConnections))
		o.ObserveInt64(inUse, int64(stats.InUse))
		o.ObserveInt64(waits, stats.WaitCount)
		o.ObserveFloat64(waited, stats.WaitDuration.Seconds())
		return nil
	}, open, inUse, waits, waited)
	return err
}

// The outcomes a sign-in is counted under.
const (
	SignInSuccess   = "success"
	SignInFailure   = "failure"
	SignInThrottled = "throttled"
	SignInBlocked   = "blocked"
)

// SignInOutcome reads a sign-in's outcome from its activity-log action
// (user.login_failed, admin.login_blocked, …) and reason; ok is false for an
// action that is not a sign-in attempt.
func SignInOutcome(action string, metadata map[string]any) (who, outcome string, ok bool) {
	who, what, found := strings.Cut(action, ".")
	if !found || (who != "user" && who != "admin") {
		return "", "", false
	}
	throttled := metadata["reason"] == "throttled"
	switch what {
	case "login":
		return who, SignInSuccess, true
	case "login_failed", "login_code_failed", "second_factor_failed", "mfa_failed":
		if throttled {
			return who, SignInThrottled, true
		}
		return who, SignInFailure, true
	case "login_blocked":
		if throttled {
			return who, SignInThrottled, true
		}
		return who, SignInBlocked, true
	}
	return "", "", false
}

// Recorded counts the attempt an activity-log action records, if it is one.
func (m *Metrics) Recorded(ctx context.Context, action string, metadata map[string]any) {
	if who, outcome, ok := SignInOutcome(action, metadata); ok {
		m.SignIn(ctx, who, outcome)
	}
}

// SignIn counts one attempt, by a "user" or an "admin".
func (m *Metrics) SignIn(ctx context.Context, who, outcome string) {
	if m == nil {
		return
	}
	m.signIns.Add(ctx, 1, metric.WithAttributes(attribute.String("who", who), attribute.String("outcome", outcome)))
}

// TokenIssued counts a token response. grant is one the server knows,
// checked before it is counted, so a client cannot invent a series.
func (m *Metrics) TokenIssued(ctx context.Context, grant, profile string) {
	if m == nil {
		return
	}
	m.tokens.Add(ctx, 1, metric.WithAttributes(attribute.String("grant_type", grant), attribute.String("profile", profile)))
}

// RefreshReuse counts a replayed refresh token.
func (m *Metrics) RefreshReuse(ctx context.Context) {
	if m == nil {
		return
	}
	m.refreshReuse.Add(ctx, 1)
}

// DPoPFailure counts a refused proof by its OAuth error code.
func (m *Metrics) DPoPFailure(ctx context.Context, code string) {
	if m == nil {
		return
	}
	m.dpopFailures.Add(ctx, 1, metric.WithAttributes(attribute.String("error", code)))
}

// The outcomes of telling an application of a logout.
const (
	LogoutDelivered = "delivered"
	LogoutRetrying  = "retrying"
	LogoutAbandoned = "abandoned"
)

// Logout counts one attempt to tell an application of a logout.
func (m *Metrics) Logout(ctx context.Context, outcome string) {
	if m == nil {
		return
	}
	m.logouts.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
}

// The outcomes of sending a message.
const (
	MailSent   = "sent"
	MailFailed = "failed"
	MailOff    = "off"
)

// Mail counts one message.
func (m *Metrics) Mail(ctx context.Context, outcome string) {
	if m == nil {
		return
	}
	m.mail.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
}

// Request records how long a request took. route is the route's template
// (/api/v1/admin/users/:id), never the path it was called with.
func (m *Metrics) Request(ctx context.Context, server, method, route string, status int, took time.Duration) {
	if m == nil {
		return
	}
	if route == "" {
		route = "unmatched"
	}
	m.requestTiming.Record(ctx, took.Seconds(), metric.WithAttributes(
		attribute.String("server", server),
		attribute.String("method", methodOf(method)),
		attribute.String("route", route),
		attribute.String("status", statusClass(status)),
	))
}

// name is a metric's name under the server's own prefix.
func name(metric string) string {
	return brand.Slug + "." + metric
}

// methodOf narrows a request's method to the standard ones: anything else
// a client sends would be a series of its own.
func methodOf(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return method
	}
	return "other"
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}
