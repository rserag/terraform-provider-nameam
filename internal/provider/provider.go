package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/rserag/terraform-provider-nameam/internal/client"
	"github.com/rserag/terraform-provider-nameam/internal/datasource/domains"
	"github.com/rserag/terraform-provider-nameam/internal/resource/dnsrecord"
)

type nameamProvider struct {
	version string
}

type ProviderConfig struct {
	Token        types.String `tfsdk:"token"`
	BaseURL      types.String `tfsdk:"base_url"`
	Timeout      types.String `tfsdk:"timeout"`
	Retries      types.Int64  `tfsdk:"retries"`
	RetryBackoff types.String `tfsdk:"retry_backoff"`
}

// New returns a new provider instance.
//
// IMPORTANT: version should come from main.go (ldflags) when built for releases.
// For local builds, it will default to "dev".
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &nameamProvider{
			version: version,
		}
	}
}

func (p *nameamProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "nameam"
	resp.Version = p.version
}

func (p *nameamProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{
		Attributes: map[string]providerschema.Attribute{
			"token": providerschema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Name.am API token. Defaults to the NAMEAM_TOKEN environment variable.",
			},
			"base_url": providerschema.StringAttribute{
				Optional:    true,
				Description: "Override API base URL (default: https://api.name.am).",
			},
			"timeout": providerschema.StringAttribute{
				Optional:    true,
				Description: "HTTP client timeout (e.g. 30s). Default: 30s.",
			},
			"retries": providerschema.Int64Attribute{
				Optional:    true,
				Description: "Retries for transient errors (429/5xx/timeouts). Default: 3.",
			},
			"retry_backoff": providerschema.StringAttribute{
				Optional:    true,
				Description: "Base retry backoff (e.g. 1s). Exponential backoff is applied. Default: 1s.",
			},
		},
	}
}

func (p *nameamProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg ProviderConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, err := providerClientOptions(cfg, os.Getenv)
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}

	api, err := client.New(opts)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Name.am client", err.Error())
		return
	}

	resp.DataSourceData = api
	resp.ResourceData = api
}

func providerClientOptions(cfg ProviderConfig, getenv func(string) string) (client.Options, error) {
	token := strings.TrimSpace(getenv("NAMEAM_TOKEN"))
	if !cfg.Token.IsNull() && !cfg.Token.IsUnknown() {
		token = strings.TrimSpace(cfg.Token.ValueString())
	}
	if token == "" {
		return client.Options{}, fmt.Errorf(`provider "nameam" requires a non-empty API token in "token" or NAMEAM_TOKEN`)
	}

	baseURL := "https://api.name.am"
	if !cfg.BaseURL.IsNull() && !cfg.BaseURL.IsUnknown() && strings.TrimSpace(cfg.BaseURL.ValueString()) != "" {
		baseURL = strings.TrimSpace(cfg.BaseURL.ValueString())
	}

	timeout, err := configuredDuration(cfg.Timeout, 30*time.Second, "timeout")
	if err != nil {
		return client.Options{}, err
	}
	backoff, err := configuredDuration(cfg.RetryBackoff, time.Second, "retry_backoff")
	if err != nil {
		return client.Options{}, err
	}

	retries := int64(3)
	if !cfg.Retries.IsNull() && !cfg.Retries.IsUnknown() {
		retries = cfg.Retries.ValueInt64()
		if retries < 0 {
			return client.Options{}, fmt.Errorf("retries must be >= 0")
		}
	}

	return client.Options{
		BaseURL:      baseURL,
		Token:        token,
		Timeout:      timeout,
		Retries:      int(retries),
		RetryBackoff: backoff,
	}, nil
}

func configuredDuration(value types.String, fallback time.Duration, name string) (time.Duration, error) {
	if value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(strings.TrimSpace(value.ValueString()))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be > 0", name)
	}
	return duration, nil
}

func (p *nameamProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		dnsrecord.New,
	}
}

func (p *nameamProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		domains.New,
	}
}
