package options

import "github.com/spf13/pflag"

// OTLP holds OpenTelemetry tracing configuration.
type OTLP struct {
	// gRPC exporter — takes precedence over HTTP when GRPCEndpoint is set.
	GRPCEndpoint string            `flag:"otlp-grpc-endpoint" cfg:"otlp_grpc_endpoint"`
	GRPCInsecure bool              `flag:"otlp-grpc-insecure" cfg:"otlp_grpc_insecure"`
	GRPCHeaders  map[string]string `flag:"otlp-grpc-headers"  cfg:"otlp_grpc_headers"`

	// HTTP exporter — used when GRPCEndpoint is empty.
	HTTPEndpoint string            `flag:"otlp-http-endpoint" cfg:"otlp_http_endpoint"`
	HTTPInsecure bool              `flag:"otlp-http-insecure" cfg:"otlp_http_insecure"`
	HTTPHeaders  map[string]string `flag:"otlp-http-headers"  cfg:"otlp_http_headers"`

	ServiceName             string            `flag:"otlp-service-name"              cfg:"otlp_service_name"`
	SampleRate              float64           `flag:"otlp-sample-rate"               cfg:"otlp_sample_rate"`
	ResourceAttributes      map[string]string `flag:"otlp-resource-attributes"       cfg:"otlp_resource_attributes"`
	CapturedRequestHeaders  []string          `flag:"otlp-captured-request-headers"  cfg:"otlp_captured_request_headers"`
	CapturedResponseHeaders []string          `flag:"otlp-captured-response-headers" cfg:"otlp_captured_response_headers"`
	SafeQueryParams         []string          `flag:"otlp-safe-query-params"         cfg:"otlp_safe_query_params"`
}

func (o *OTLP) SetDefaults() {
	o.ServiceName = "oauth2-proxy"
	o.SampleRate = 1.0
	o.HTTPEndpoint = "https://localhost:4318"
}

func otlpFlagSet() *pflag.FlagSet {
	flagSet := pflag.NewFlagSet("otlp", pflag.ExitOnError)

	// gRPC exporter
	flagSet.String("otlp-grpc-endpoint", "", "gRPC OTLP collector endpoint (host:port); takes precedence over HTTP when set")
	flagSet.Bool("otlp-grpc-insecure", false, "disable TLS for the gRPC OTLP connection")
	flagSet.StringToString("otlp-grpc-headers", map[string]string{}, "headers sent with gRPC OTLP payloads")

	// HTTP exporter
	flagSet.String("otlp-http-endpoint", "https://localhost:4318", "HTTP OTLP collector endpoint (scheme://host:port/path)")
	flagSet.Bool("otlp-http-insecure", false, "disable TLS for the HTTP OTLP connection")
	flagSet.StringToString("otlp-http-headers", map[string]string{}, "headers sent with HTTP OTLP payloads")

	// Common
	flagSet.String("otlp-service-name", "oauth2-proxy", "service name reported to the OTLP collector")
	flagSet.Float64("otlp-sample-rate", 1.0, "trace sampling rate (0.0–1.0)")
	flagSet.StringToString("otlp-resource-attributes", map[string]string{}, "additional resource attributes sent to the OTLP collector")
	flagSet.StringSlice("otlp-captured-request-headers", []string{}, "request headers to capture as span attributes")
	flagSet.StringSlice("otlp-captured-response-headers", []string{}, "response headers to capture as span attributes")
	flagSet.StringSlice("otlp-safe-query-params", []string{}, "query parameters that are not redacted in span URL attributes")

	return flagSet
}

func otlpDefaults() OTLP {
	o := OTLP{}
	o.SetDefaults()
	return o
}
