package validation

import (
	"fmt"
	"net"
	"net/url"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/logger"
)

func validateOTLP(o options.OTLP) []string {
	// No endpoint configured at all — tracing is disabled, nothing to validate.
	if o.GRPCEndpoint == "" && o.HTTPEndpoint == "" {
		return nil
	}

	var msgs []string

	if o.GRPCEndpoint != "" {
		msgs = append(msgs, validateGRPCEndpoint(o.GRPCEndpoint)...)
		if o.GRPCInsecure {
			logger.Printf("WARNING: OTLP gRPC insecure mode is enabled (should only be used for development)")
		}
	} else {
		msgs = append(msgs, validateHTTPEndpoint(o.HTTPEndpoint)...)
		if o.HTTPInsecure {
			logger.Printf("WARNING: OTLP HTTP insecure mode is enabled (should only be used for development)")
		}
	}

	if o.SampleRate < 0 || o.SampleRate > 1 {
		msgs = append(msgs, fmt.Sprintf("otlp_sample_rate (%v) must be between 0.0 and 1.0", o.SampleRate))
	}

	return msgs
}

func validateGRPCEndpoint(endpoint string) []string {
	if endpoint == "" {
		return nil
	}
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		return []string{fmt.Sprintf(
			"invalid otlp_grpc_endpoint %q: must be in host:port format", endpoint,
		)}
	}
	return nil
}

func validateHTTPEndpoint(endpoint string) []string {
	if endpoint == "" {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return []string{fmt.Sprintf(
			"invalid otlp_http_endpoint %q: must be a valid http:// or https:// URL", endpoint,
		)}
	}
	return nil
}
