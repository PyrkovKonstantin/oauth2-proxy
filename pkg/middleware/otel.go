package middleware

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// NewTracingMiddleware returns a mux middleware that starts a server span for
// every incoming request, following the same pattern as Traefik's entrypoint
// tracing middleware.
func NewTracingMiddleware(tracer *observability.Tracer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Follow OTEL semantic conventions for HTTP span names:
			// use the HTTP method as the span name since the route template
			// is not always low-cardinality.
			tracingCtx := observability.ExtractCarrierIntoContext(r.Context(), r.Header)
			start := time.Now()

			tracingCtx, span := tracer.Start(
				tracingCtx,
				r.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithTimestamp(start),
			)
			defer func() {
				span.End(trace.WithTimestamp(time.Now()))
			}()

			// Enrich span with HTTP server attributes.
			tracer.CaptureServerRequest(span, r)

			// If we can resolve the matched route, record it as an attribute.
			if route := mux.CurrentRoute(r); route != nil {
				if tmpl, err := route.GetPathTemplate(); err == nil {
					span.SetAttributes(attribute.String("http.route", tmpl))
				}
			}

			rw := newStatusRecorder(w)
			next.ServeHTTP(rw, r.WithContext(tracingCtx))

			tracer.CaptureResponse(span, rw.Header(), rw.status, trace.SpanKindServer)
		})
	}
}

// statusRecorder wraps http.ResponseWriter to capture the written status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w}
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}
