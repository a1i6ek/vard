package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/a1i6ek/vard/internal/brand"
	"github.com/a1i6ek/vard/internal/telemetry"
)

// Observe times each request by its route template and, when traces are
// sent anywhere, wraps it in a span. The span starts a trace of its own: a
// traceparent from a caller is not taken, so nobody outside picks the ids
// an operator's traces are filed under.
func Observe(server string, metrics *telemetry.Metrics) gin.HandlerFunc {
	tracer := otel.Tracer(brand.Slug)

	return func(c *gin.Context) {
		start := time.Now()
		route := c.FullPath()

		ctx, span := tracer.Start(c.Request.Context(), c.Request.Method+" "+route,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithNewRoot(),
			trace.WithAttributes(
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("http.route", route),
				attribute.String("server", server),
			))
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		status := c.Writer.Status()
		span.SetAttributes(attribute.Int("http.response.status_code", status))
		span.End()

		metrics.Request(ctx, server, c.Request.Method, route, status, time.Since(start))
	}
}
