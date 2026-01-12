package provider

import (
	"context"
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
	RateLimit    types.Float64 `tfsdk:"rate_limit"`
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
				Required:  true,
				Sensitive: true,
				// Description is optional; keeping it short.
				Description: "Name.am JWT bearer token.",
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
			"rate_limit": providerschema.Float64Attribute{
				Optional:    true,
				Description: "Maximum number of API requests per second. Default: 10. Set to 0 to disable rate limiting.",
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

	// Validate token
	if cfg.Token.IsNull() || cfg.Token.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing token",
			`Provider "nameam" requires a non-empty "token" (JWT bearer token).`,
		)
		return
	}

	baseURL := "https://api.name.am"
	if !cfg.BaseURL.IsNull() && cfg.BaseURL.ValueString() != "" {
		baseURL = cfg.BaseURL.ValueString()
	}

	timeout := 30 * time.Second
	if !cfg.Timeout.IsNull() && cfg.Timeout.ValueString() != "" {
		d, err := time.ParseDuration(cfg.Timeout.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid timeout", err.Error())
			return
		}
		if d <= 0 {
			resp.Diagnostics.AddError("Invalid timeout", "timeout must be > 0")
			return
		}
		timeout = d
	}

	retries := int64(3)
	if !cfg.Retries.IsNull() {
		retries = cfg.Retries.ValueInt64()
		if retries < 0 {
			resp.Diagnostics.AddError("Invalid retries", "retries must be >= 0")
			return
		}
	}

	backoff := 1 * time.Second
	if !cfg.RetryBackoff.IsNull() && cfg.RetryBackoff.ValueString() != "" {
		d, err := time.ParseDuration(cfg.RetryBackoff.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid retry_backoff", err.Error())
			return
		}
		if d <= 0 {
			resp.Diagnostics.AddError("Invalid retry_backoff", "retry_backoff must be > 0")
			return
		}
		backoff = d
	}

	rateLimit := 10.0 // Default: 10 requests per second
	if !cfg.RateLimit.IsNull() {
		rateLimit = cfg.RateLimit.ValueFloat64()
		if rateLimit < 0 {
			resp.Diagnostics.AddError("Invalid rate_limit", "rate_limit must be >= 0")
			return
		}
	}

	api, err := client.New(client.Options{
		BaseURL:      baseURL,
		Token:        cfg.Token.ValueString(),
		Timeout:      timeout,
		Retries:      int(retries),
		RetryBackoff: backoff,
		RateLimit:    rateLimit,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Name.am client", err.Error())
		return
	}

	resp.DataSourceData = api
	resp.ResourceData = api
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
