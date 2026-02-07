package observability

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	ctxKeyRequestID = "request_id"
	ctxKeyTraceID   = "trace_id"
)

// RequestContextMiddleware ensures each request has a request id and captures trace id for log correlation.
func RequestContextMiddleware() gin.HandlerFunc {
	projectID := strings.TrimSpace(os.Getenv("GCP_PROJECT_ID"))

	return func(c *gin.Context) {
		reqID := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		if reqID == "" {
			reqID = newRequestID()
		}
		c.Set(ctxKeyRequestID, reqID)
		c.Writer.Header().Set("X-Request-Id", reqID)

		traceID := ExtractTraceID(c.Request)
		if traceID != "" {
			c.Set(ctxKeyTraceID, traceID)
		}

		logger := slog.Default().With(
			slog.String("request_id", reqID),
		)
		if traceID != "" {
			if trace := CloudLoggingTrace(projectID, traceID); trace != "" {
				logger = logger.With(slog.String("logging.googleapis.com/trace", trace))
			}
		}
		c.Set("logger", logger)

		c.Next()
	}
}

// AccessLogMiddleware emits a structured access log per request.
func AccessLogMiddleware() gin.HandlerFunc {
	projectID := strings.TrimSpace(os.Getenv("GCP_PROJECT_ID"))

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		status := c.Writer.Status()
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		reqID := getString(c, ctxKeyRequestID)
		traceID := getString(c, ctxKeyTraceID)

		logger := slog.Default()
		if v, ok := c.Get("logger"); ok {
			if l, ok := v.(*slog.Logger); ok && l != nil {
				logger = l
			}
		}

		attrs := []slog.Attr{
			slog.String("method", method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Int64("latency_ms", latency.Milliseconds()),
		}

		if reqID != "" {
			attrs = append(attrs, slog.String("request_id", reqID))
		}
		if traceID != "" {
			if trace := CloudLoggingTrace(projectID, traceID); trace != "" {
				attrs = append(attrs, slog.String("logging.googleapis.com/trace", trace))
			}
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		logger.LogAttrs(c.Request.Context(), level, "http_request", attrs...)
	}
}

func getString(c *gin.Context, key string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return hex.EncodeToString(b[:])
}

