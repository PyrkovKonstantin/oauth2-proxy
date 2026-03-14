package observability

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/encoding/gzip"
)

const tracerName = "github.com/oauth2-proxy/oauth2-proxy"

// Tracer wraps trace.Tracer with additional configuration for capturing HTTP
// request/response attributes on spans. It follows the same pattern as Traefik.
type Tracer struct {
	trace.Tracer

	safeQueryParams         []string
	capturedRequestHeaders  []string
	capturedResponseHeaders []string
}

// Span wraps trace.Span to carry our TracerProvider so TracerFromContext works.
type Span struct {
	trace.Span
	tracerProvider *TracerProvider
}

// TracerProvider returns the span's TracerProvider.
func (s Span) TracerProvider() trace.TracerProvider {
	return s.tracerProvider
}

// TracerProvider wraps trace.TracerProvider to return our Tracer when requested
// by name, allowing TracerFromContext to retrieve it from any span in context.
type TracerProvider struct {
	trace.TracerProvider
	tracer *Tracer
}

// Tracer returns our Tracer when the name matches; falls back to the underlying provider.
func (tp TracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if name == tracerName {
		return tp.tracer
	}
	return tp.TracerProvider.Tracer(name, opts...)
}

// NewTracer builds a Tracer with captured headers and safe query params.
func NewTracer(t trace.Tracer, capturedRequestHeaders, capturedResponseHeaders, safeQueryParams []string) *Tracer {
	return &Tracer{
		Tracer:                  t,
		safeQueryParams:         safeQueryParams,
		capturedRequestHeaders:  canonicalizeHeaders(capturedRequestHeaders),
		capturedResponseHeaders: canonicalizeHeaders(capturedResponseHeaders),
	}
}

// NoopTracer returns a Tracer that records nothing. Used when tracing is disabled.
func NoopTracer() *Tracer {
	return NewTracer(noop.NewTracerProvider().Tracer(tracerName), nil, nil, nil)
}

// Start begins a new span, wrapping it so TracerFromContext can retrieve this Tracer.
func (t *Tracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if t == nil {
		return ctx, nil
	}
	spanCtx, span := t.Tracer.Start(ctx, spanName, opts...)
	wrapped := &Span{
		Span:           span,
		tracerProvider: &TracerProvider{TracerProvider: span.TracerProvider(), tracer: t},
	}
	return trace.ContextWithSpan(spanCtx, wrapped), wrapped
}

// TracerFromContext extracts the Tracer from the span stored in ctx.
// Returns nil when no valid span is present (e.g. tracing is disabled).
func TracerFromContext(ctx context.Context) *Tracer {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return nil
	}
	span := trace.SpanFromContext(ctx)
	if span == nil || span.TracerProvider() == nil {
		return nil
	}
	if t, ok := span.TracerProvider().Tracer(tracerName).(*Tracer); ok {
		return t
	}
	return nil
}

// ExtractCarrierIntoContext reads trace context from the request headers into ctx.
func ExtractCarrierIntoContext(ctx context.Context, headers http.Header) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(headers))
}

// InjectContextIntoCarrier writes the current trace context into the request headers.
func InjectContextIntoCarrier(req *http.Request) {
	otel.GetTextMapPropagator().Inject(req.Context(), propagation.HeaderCarrier(req.Header))
}

// CaptureServerRequest sets HTTP server semantic convention attributes on span.
func (t *Tracer) CaptureServerRequest(span trace.Span, r *http.Request) {
	if t == nil || span == nil || r == nil {
		return
	}

	span.SetAttributes(semconv.HTTPRequestMethodKey.String(r.Method))
	span.SetAttributes(semconv.NetworkProtocolVersion(protoVersion(r.Proto)))

	safeURL := t.safeURL(r.URL)
	span.SetAttributes(semconv.HTTPRequestBodySize(int(r.ContentLength)))
	span.SetAttributes(semconv.URLPath(safeURL.Path))
	span.SetAttributes(semconv.URLQuery(safeURL.RawQuery))
	span.SetAttributes(semconv.URLScheme(r.Header.Get("X-Forwarded-Proto")))
	span.SetAttributes(semconv.UserAgentOriginal(r.UserAgent()))
	span.SetAttributes(semconv.ServerAddress(r.Host))

	host, port, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		span.SetAttributes(semconv.ClientAddress(r.RemoteAddr))
		span.SetAttributes(semconv.NetworkPeerAddress(r.RemoteAddr))
	} else {
		span.SetAttributes(semconv.ClientAddress(host))
		span.SetAttributes(semconv.NetworkPeerAddress(host))
		if p, err := strconv.Atoi(port); err == nil {
			span.SetAttributes(semconv.ClientPort(p))
			span.SetAttributes(semconv.NetworkPeerPort(p))
		}
	}

	for _, header := range t.capturedRequestHeaders {
		if strings.EqualFold(header, "User-Agent") {
			continue // already captured via semconv
		}
		if vals := r.Header[header]; vals != nil {
			span.SetAttributes(attribute.StringSlice(
				fmt.Sprintf("http.request.header.%s", strings.ToLower(header)), vals,
			))
		}
	}
}

// CaptureResponse sets HTTP response status attributes and the span status on span.
func (t *Tracer) CaptureResponse(span trace.Span, responseHeaders http.Header, code int, spanKind trace.SpanKind) {
	if t == nil || span == nil {
		return
	}

	var status codes.Code
	var desc string
	switch spanKind {
	case trace.SpanKindServer:
		status, desc = serverStatus(code)
	case trace.SpanKindClient:
		status, desc = clientStatus(code)
	default:
		status, desc = serverStatus(code)
	}
	span.SetStatus(status, desc)
	if code > 0 {
		span.SetAttributes(semconv.HTTPResponseStatusCode(code))
	}

	for _, header := range t.capturedResponseHeaders {
		if vals := responseHeaders[header]; vals != nil {
			span.SetAttributes(attribute.StringSlice(
				fmt.Sprintf("http.response.header.%s", strings.ToLower(header)), vals,
			))
		}
	}
}

// InitProvider initialises the OTel trace provider from cfg and returns a Tracer and closer.
func InitProvider(ctx context.Context, cfg *options.OTLP) (*Tracer, io.Closer, error) {
	var (
		exporter *otlptrace.Exporter
		err      error
	)

	if cfg.GRPCEndpoint != "" {
		exporter, err = setupGRPCExporter(cfg)
	} else {
		exporter, err = setupHTTPExporter(cfg)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("setting up OTLP exporter: %w", err)
	}

	var resAttrs []attribute.KeyValue
	for k, v := range cfg.ResourceAttributes {
		resAttrs = append(resAttrs, attribute.String(k, v))
	}

	res, err := resource.New(ctx,
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(version.VERSION),
		),
		resource.WithAttributes(resAttrs...),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("building OTLP resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := NewTracer(
		tp.Tracer(tracerName),
		cfg.CapturedRequestHeaders,
		cfg.CapturedResponseHeaders,
		cfg.SafeQueryParams,
	)

	return tracer, &tpCloser{provider: tp}, nil
}

func setupHTTPExporter(cfg *options.OTLP) (*otlptrace.Exporter, error) {
	endpoint, err := url.Parse(cfg.HTTPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP endpoint %q: %w", cfg.HTTPEndpoint, err)
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint.Host),
		otlptracehttp.WithHeaders(cfg.HTTPHeaders),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
	}
	if endpoint.Scheme == "http" || cfg.HTTPInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if endpoint.Path != "" {
		opts = append(opts, otlptracehttp.WithURLPath(endpoint.Path))
	}

	return otlptrace.New(context.Background(), otlptracehttp.NewClient(opts...))
}

func setupGRPCExporter(cfg *options.OTLP) (*otlptrace.Exporter, error) {
	host, port, err := net.SplitHostPort(cfg.GRPCEndpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid gRPC endpoint %q: %w", cfg.GRPCEndpoint, err)
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(fmt.Sprintf("%s:%s", host, port)),
		otlptracegrpc.WithHeaders(cfg.GRPCHeaders),
		otlptracegrpc.WithCompressor(gzip.Name),
	}
	if cfg.GRPCInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	return otlptrace.New(context.Background(), otlptracegrpc.NewClient(opts...))
}

// tpCloser shuts down a TracerProvider with a deadline.
type tpCloser struct {
	provider *sdktrace.TracerProvider
}

func (c *tpCloser) Close() error {
	if c == nil {
		return nil
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()
	return c.provider.Shutdown(ctx)
}

// safeURL returns a copy of u with credentials and unsafe query params redacted.
func (t *Tracer) safeURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	safe := *u
	if safe.User != nil {
		safe.User = url.UserPassword("REDACTED", "REDACTED")
	}
	q := safe.Query()
	for k := range q {
		if !slices.Contains(t.safeQueryParams, k) {
			q.Set(k, "REDACTED")
		}
	}
	safe.RawQuery = q.Encode()
	return &safe
}

// serverStatus maps an HTTP response code to a span status for a server span.
// 4xx are not errors from the server's perspective.
func serverStatus(code int) (codes.Code, string) {
	if code < 100 || code >= 600 {
		return codes.Error, fmt.Sprintf("Invalid HTTP status code %d", code)
	}
	if code >= 500 {
		return codes.Error, ""
	}
	return codes.Unset, ""
}

// clientStatus maps an HTTP response code to a span status for a client span.
// 4xx are errors from the client's perspective.
func clientStatus(code int) (codes.Code, string) {
	if code < 100 || code >= 600 {
		return codes.Error, fmt.Sprintf("Invalid HTTP status code %d", code)
	}
	if code >= 400 {
		return codes.Error, ""
	}
	return codes.Unset, ""
}

func protoVersion(proto string) string {
	switch proto {
	case "HTTP/1.0":
		return "1.0"
	case "HTTP/1.1":
		return "1.1"
	case "HTTP/2":
		return "2"
	case "HTTP/3":
		return "3"
	default:
		return proto
	}
}

func canonicalizeHeaders(headers []string) []string {
	if headers == nil {
		return nil
	}
	out := make([]string, len(headers))
	for i, h := range headers {
		out[i] = http.CanonicalHeaderKey(h)
	}
	return out
}
