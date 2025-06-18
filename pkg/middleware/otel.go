package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/semconv/v1.17.0/httpconv"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func OtelMiddleware(appName string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Printf("🔵 [OtelMiddleware] Start processing request: %s %s", r.Method, r.URL.Path)

			tracerProvider := otel.GetTracerProvider()
			logger.Printf("  ├─ [Tracing] TracerProvider obtained: %T", tracerProvider)

			tracer := tracerProvider.Tracer("main")
			logger.Printf("  ├─ [Tracing] Tracer created: %s", "main")

			savedCtx := r.Context()
			logger.Printf("  ├─ [Context] Original context saved")

			defer func() {
				r = r.WithContext(savedCtx)
				logger.Printf("  └─ [Context] Original context restored in defer")
			}()

			propagator := otel.GetTextMapPropagator()
			ctx := propagator.Extract(savedCtx, propagation.HeaderCarrier(r.Header))
			logger.Printf("  ├─ [Propagation] Context extracted from headers. TraceID: %s", getTraceID(ctx))

			opts := []oteltrace.SpanStartOption{
				oteltrace.WithAttributes(httpconv.ServerRequest(appName, r)...),
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			}
			logger.Printf("  ├─ [Span] Start options prepared: %d options", len(opts))

			var routePath string
			if route := mux.CurrentRoute(r); route != nil {
				if path, err := route.GetPathTemplate(); err == nil {
					routePath = path
					opts = append(opts, oteltrace.WithAttributes(semconv.HTTPRoute(routePath)))
					logger.Printf("  ├─ [Routing] Matched route path: %s", routePath)
				}
			}

			spanName := routePath
			if spanName == "" {
				spanName = fmt.Sprintf("HTTP %s route not found", r.Method)
				logger.Printf("  ├─ [Routing] No route matched, using fallback span name: %s", spanName)
			}

			ctx, span := tracer.Start(ctx, spanName, opts...)
			logger.Printf("  ├─ [Span] Started: %s (TraceID: %s, SpanID: %s)",
				spanName, getTraceID(ctx), getSpanID(ctx))
			defer func() {
				span.End()
				logger.Printf("  ├─ [Span] Ended: %s (TraceID: %s)", spanName, getTraceID(ctx))
			}()

			r = r.WithContext(ctx)
			logger.Printf("  ├─ [Context] Request context updated with new span")

			rw := &responseWriter{w: w, status: 0}
			logger.Printf("  ├─ [Response] Wrapper created")

			logger.Printf("  ├─ [Chain] Calling next handler...")
			next.ServeHTTP(rw, r)
			logger.Printf("  ├─ [Chain] Next handler completed")

			if rw.status > 0 {
				span.SetAttributes(semconv.HTTPStatusCode(rw.status))
				span.SetStatus(httpconv.ServerStatus(rw.status))
				logger.Printf("  ├─ [Response] Status code recorded: %d", rw.status)
			}

			logger.Printf("🟢 [OtelMiddleware] Finished processing request: %s %s", r.Method, r.URL.Path)
		})
	}
}

func getTraceID(ctx context.Context) string {
	if span := oteltrace.SpanFromContext(ctx); span != nil {
		return span.SpanContext().TraceID().String()
	}
	return "no-trace-id"
}

func getSpanID(ctx context.Context) string {
	if span := oteltrace.SpanFromContext(ctx); span != nil {
		return span.SpanContext().SpanID().String()
	}
	return "no-span-id"
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
