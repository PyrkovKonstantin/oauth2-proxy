package validation

import (
	"fmt"
	"net"
	"strings"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/logger"
)

func validateOTLP(o options.OTLP) []string {
	var msgs []string

	if o.Endpoint == "" && o.Protocol == "" {
		return msgs
	}
	// Validate OTLP endpoint
	msgs = append(msgs, validateOTLPEndpoint(o.Endpoint)...)

	// Validate OTLP protocol
	msgs = append(msgs, validateOTLPProtocol(o.Protocol)...)

	// Insecure is boolean, no validation needed but we can log if it's true
	if o.Insecure {
		logger.Printf("WARNING: OTLP insecure mode is enabled (should only be used for development)")
	}

	return msgs
}

func validateOTLPEndpoint(endpoint string) []string {
	var msgs []string

	if endpoint == "" {
		return []string{"missing setting: otlp_endpoint"}
	}

	// Check if endpoint has port
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		msgs = append(msgs, fmt.Sprintf(
			"invalid otlp_endpoint format %q: must be in format 'host:port'",
			endpoint))
	}

	// Basic DNS validation
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		msgs = append(msgs, fmt.Sprintf(
			"invalid otlp_endpoint %q: should not include scheme (http/https)",
			endpoint))
	}

	return msgs
}

func validateOTLPProtocol(protocol string) []string {
	switch strings.ToLower(protocol) {
	case "", "http", "grpc":
		return []string{}
	default:
		return []string{fmt.Sprintf(
			"otlp_protocol (%q) must be one of ['', 'http', 'grpc']",
			protocol)}
	}
}
