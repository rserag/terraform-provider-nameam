package domains

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/rserag/terraform-provider-nameam/internal/client"
)

type dataSource struct {
	api *client.Client
}

type domainsModel struct {
	Domains []domainModel `tfsdk:"domains"`
}

type domainModel struct {
	Domain      types.String   `tfsdk:"domain"`
	DnsUsed     types.Bool     `tfsdk:"dns_used"`
	NameServers []types.String `tfsdk:"nameservers"`
}

func New() datasource.DataSource {
	return &dataSource{}
}

func (d *dataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (d *dataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"domains": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"domain":   schema.StringAttribute{Computed: true},
						"dns_used": schema.BoolAttribute{Computed: true},
						"nameservers": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

func (d *dataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.api = req.ProviderData.(*client.Client)
}

func (d *dataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	r, err := d.api.GetDomains(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list domains", err.Error())
		return
	}

	out := domainsToModel(r.Docs)

	diags := resp.State.Set(ctx, &out)
	resp.Diagnostics.Append(diags...)
}

func domainsToModel(domains []client.Domain) domainsModel {
	out := domainsModel{Domains: make([]domainModel, 0, len(domains))}
	for _, dom := range domains {
		ns := make([]types.String, 0, len(dom.NameServers))
		for _, n := range dom.NameServers {
			if strings.TrimSpace(n.Hostname) == "" {
				continue
			}
			ns = append(ns, types.StringValue(n.Hostname))
		}
		out.Domains = append(out.Domains, domainModel{
			Domain:      types.StringValue(strings.ToLower(dom.Domain)),
			DnsUsed:     types.BoolValue(dom.DnsUsed),
			NameServers: ns,
		})
	}
	return out
}
