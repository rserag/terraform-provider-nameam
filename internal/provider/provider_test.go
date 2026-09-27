package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestProviderSchemaIsValid(t *testing.T) {
	t.Parallel()
	server, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("provider schema error: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
}

func TestProviderClientOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		config    ProviderConfig
		envToken  string
		wantToken string
		wantBase  string
		wantRetry int
		wantErr   string
	}{
		{
			name: "defaults with environment token", envToken: " env-token ",
			wantToken: "env-token", wantBase: "https://api.name.am", wantRetry: 3,
		},
		{
			name: "explicit values including zero retries",
			config: ProviderConfig{
				Token: types.StringValue(" config-token "), BaseURL: types.StringValue(" https://example.test/ "),
				Timeout: types.StringValue("5s"), Retries: types.Int64Value(0), RetryBackoff: types.StringValue("250ms"),
			},
			envToken: "ignored", wantToken: "config-token", wantBase: "https://example.test/", wantRetry: 0,
		},
		{name: "missing token", wantErr: "requires a non-empty API token"},
		{name: "negative retries", envToken: "token", config: ProviderConfig{Retries: types.Int64Value(-1)}, wantErr: "retries must be >= 0"},
		{name: "invalid timeout", envToken: "token", config: ProviderConfig{Timeout: types.StringValue("soon")}, wantErr: "invalid timeout"},
		{name: "non-positive backoff", envToken: "token", config: ProviderConfig{RetryBackoff: types.StringValue("0s")}, wantErr: "retry_backoff must be > 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := providerClientOptions(tt.config, func(string) string { return tt.envToken })
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error=%v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if opts.Token != tt.wantToken || opts.BaseURL != tt.wantBase || opts.Retries != tt.wantRetry {
				t.Fatalf("options=%+v", opts)
			}
			if tt.name == "defaults with environment token" && (opts.Timeout != 30*time.Second || opts.RetryBackoff != time.Second) {
				t.Fatalf("default durations=%s/%s", opts.Timeout, opts.RetryBackoff)
			}
		})
	}
}
