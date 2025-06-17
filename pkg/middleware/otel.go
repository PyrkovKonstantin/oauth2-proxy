package middleware

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/semconv/v1.17.0/httpconv"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func OtelMiddleware(appName string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracerProvider := otel.GetTracerProvider()
			tracer := tracerProvider.Tracer("main")

			savedCtx := r.Context()

			defer func() {
				r = r.WithContext(savedCtx)
			}()

			propagator := otel.GetTextMapPropagator()
			ctx := propagator.Extract(savedCtx, propagation.HeaderCarrier(r.Header))

			opts := []oteltrace.SpanStartOption{
				oteltrace.WithAttributes(httpconv.ServerRequest(appName, r)...),
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			}

			var routePath string
			if route := mux.CurrentRoute(r); route != nil {
				if path, err := route.GetPathTemplate(); err == nil {
					routePath = path
					opts = append(opts, oteltrace.WithAttributes(semconv.HTTPRoute(routePath)))
				}
			}

			spanName := routePath
			if spanName == "" {
				spanName = fmt.Sprintf("HTTP %s route not found", r.Method)
			}

			ctx, span := tracer.Start(ctx, spanName, opts...)
			defer span.End()

			r = r.WithContext(ctx)

			rw := &responseWriter{w: w, status: 0}

			next.ServeHTTP(rw, r)

			if rw.status > 0 {
				span.SetAttributes(semconv.HTTPStatusCode(rw.status))
				span.SetStatus(httpconv.ServerStatus(rw.status))
			}
		})
	}
}

type responseWriter struct {
	w      http.ResponseWriter
	status int
}

func (rw *responseWriter) Header() http.Header {
	return rw.w.Header()
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	return rw.w.Write(b)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.w.WriteHeader(statusCode)
}
