package observability

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const TraceIDHeader = "X-Trace-ID"

// Middleware returns a Gin middleware that:
// - generates a trace ID per request (or uses an incoming X-Trace-ID header)
// - records Prometheus metrics (request count, latency, active connections)
// - emits a structured JSON log line per request
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(TraceIDHeader)
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Header(TraceIDHeader, traceID)

		ctx := ContextWithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		path := normalizedPath(c)

		HTTPActiveRequests.Inc()
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		statusStr := strconv.Itoa(status)

		HTTPActiveRequests.Dec()
		HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, statusStr).Inc()
		HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration.Seconds())

		slog.InfoContext(ctx, "request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.String("duration", fmt.Sprintf("%.3fms", float64(duration.Microseconds())/1000)),
			slog.String("client_ip", c.ClientIP()),
		)
	}
}

// normalizedPath returns the route template (e.g. /api/v1/buckets/:id) to avoid
// high-cardinality label values in Prometheus metrics.
func normalizedPath(c *gin.Context) string {
	route := c.FullPath()
	if route == "" {
		return "unmatched"
	}
	return route
}
