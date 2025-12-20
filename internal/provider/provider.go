package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
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

func New() provider.Provider {
	return &nameamProvider{
		version: "0.1.0",
	}
}

func (p *nameamProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "nameam"
	resp.Version = p.version
}

func (p *nameamProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = pschema.Schema{
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
			},
			"base_url": schema.StringAttribute{
				Optional: true,
			},
			"timeout": schema.StringAttribute{
				Optional:    true,
				Description: "HTTP client timeout (e.g. 30s).",
			},
			"retries": schema.Int64Attribute{
				Optional:    true,
				Description: "Retries for transient errors (429/5xx/timeouts).",
			},
			"retry_backoff": schema.StringAttribute{
				Optional:    true,
				Description: "Base retry backoff (e.g. 1s). Exponential backoff is applied.",
			},
		},
	}
}

func (p *nameamProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg ProviderConfig
	diags := req.Config.Get(ctx, &cfg)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := "https://api.name.am"
	if !cfg.BaseURL.IsNull() && cfg.BaseURL.ValueString() != "" {
		baseURL = cfg.BaseURL.ValueString()
	}

	timeout := 30 * time.Second
	if !cfg.Timeout.IsNull() && cfg.Timeout.ValueString() != "" {
		if d, err := time.ParseDuration(cfg.Timeout.ValueString()); err == nil && d > 0 {
			timeout = d
		} else if err != nil {
			resp.Diagnostics.AddError("Invalid timeout", err.Error())
			return
		}
	}

	retries := int64(3)
	if !cfg.Retries.IsNull() {
		retries = cfg.Retries.ValueInt64()
	}

	backoff := 1 * time.Second
	if !cfg.RetryBackoff.IsNull() && cfg.RetryBackoff.ValueString() != "" {
		if d, err := time.ParseDuration(cfg.RetryBackoff.ValueString()); err == nil && d > 0 {
			backoff = d
		} else if err != nil {
			resp.Diagnostics.AddError("Invalid retry_backoff", err.Error())
			return
		}
	}

	api, err := client.New(client.Options{
		BaseURL:      baseURL,
		Token:        cfg.Token.ValueString(),
		Timeout:      timeout,
		Retries:      int(retries),
		RetryBackoff: backoff,
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
