package options

import (
	"time"

	"github.com/spf13/pflag"
)

type OTLP struct {
	Endpoint       string        `flag:"otlp-endpoint" cfg:"otlp_endpoint"`
	Protocol       string        `flag:"otlp-protocol" cfg:"otlp_protocol"`
	Insecure       bool          `flag:"otlp-insecure" cfg:"otlp_insecure"`
	SamplingRate   float64       `flag:"otlp-sampling-rate" cfg:"otlp_sampling_rate"`
	Headers        []string      `flag:"otlp-headers" cfg:"otlp_headers"`
	Timeout        time.Duration `flag:"otlp-timeout" cfg:"otlp_timeout"`
	Compression    string        `flag:"otlp-compression" cfg:"otlp_compression"`
	Certificate    string        `flag:"otlp-certificate" cfg:"otlp_certificate"`
	ClientKey      string        `flag:"otlp-client-key" cfg:"otlp_client_key"`
	ClientCert     string        `flag:"otlp-client-cert" cfg:"otlp_client_cert"`
	TracesEndpoint string        `flag:"otlp-traces-endpoint" cfg:"otlp_traces_endpoint"`
	TracesHeaders  []string      `flag:"otlp-traces-headers" cfg:"otlp_traces_headers"`
	TracesTimeout  time.Duration `flag:"otlp-traces-timeout" cfg:"otlp_traces_timeout"`
	TracesCompress string        `flag:"otlp-traces-compression" cfg:"otlp_traces_compression"`
}

func otlpFlagSet() *pflag.FlagSet {
	flagSet := pflag.NewFlagSet("otlp", pflag.ExitOnError)

	flagSet.String("otlp-endpoint", "https://localhost:4318", "OTLP endpoint base URL (scheme://host:port)")
	flagSet.String("otlp-protocol", "grpc", "OTLP protocol (grpc|http/protobuf)")
	flagSet.Bool("otlp-insecure", false, "disable TLS for OTLP connection")
	flagSet.Float64("otlp-sampling-rate", 1.0, "sampling rate for traces (0.0 - 1.0)")
	flagSet.StringSlice("otlp-headers", []string{}, "OTLP headers (key=value pairs)")
	flagSet.Duration("otlp-timeout", 10*time.Second, "OTLP export timeout")
	flagSet.String("otlp-compression", "", "OTLP compression (gzip)")
	flagSet.String("otlp-certificate", "", "path to TLS certificate")
	flagSet.String("otlp-client-key", "", "path to client private key")
	flagSet.String("otlp-client-cert", "", "path to client certificate")
	flagSet.String("otlp-traces-endpoint", "", "override OTLP traces endpoint")
	flagSet.StringSlice("otlp-traces-headers", []string{}, "override OTLP traces headers")
	flagSet.Duration("otlp-traces-timeout", 0, "override OTLP traces timeout")
	flagSet.String("otlp-traces-compression", "", "override OTLP traces compression")

	return flagSet
}

func otlpDefaults() OTLP {
	return OTLP{
		Endpoint:       "",
		Protocol:       "",
		Insecure:       true,
		SamplingRate:   1.0,
		Headers:        nil,
		Timeout:        10 * time.Second,
		Compression:    "",
		Certificate:    "",
		ClientKey:      "",
		ClientCert:     "",
		TracesEndpoint: "",
		TracesHeaders:  nil,
		TracesTimeout:  0,
		TracesCompress: "",
	}
}
