package observability

import (
	"net/http"
	"strings"
)

// ExtractTraceID attempts to extract a trace id from common headers.
// - X-Cloud-Trace-Context: TRACE_ID/SPAN_ID;o=1
// - traceparent: 00-TRACE_ID-SPAN_ID-FLAGS
func ExtractTraceID(r *http.Request) string {
	if r == nil {
		return ""
	}

	if h := strings.TrimSpace(r.Header.Get("X-Cloud-Trace-Context")); h != "" {
		if i := strings.IndexByte(h, '/'); i > 0 {
			return h[:i]
		}
	}

	if h := strings.TrimSpace(r.Header.Get("traceparent")); h != "" {
		parts := strings.Split(h, "-")
		if len(parts) >= 4 && len(parts[1]) == 32 {
			return parts[1]
		}
	}

	return ""
}

func CloudLoggingTrace(projectID, traceID string) string {
	projectID = strings.TrimSpace(projectID)
	traceID = strings.TrimSpace(traceID)
	if projectID == "" || traceID == "" {
		return ""
	}
	return "projects/" + projectID + "/traces/" + traceID
}

