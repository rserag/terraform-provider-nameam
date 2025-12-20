package dnsrecord

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	rplan "github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	splan "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/rserag/terraform-provider-nameam/internal/client"
)

type resourceDNSRecord struct {
	api *client.Client
}

type dnsRecordModel struct {
	Domain   types.String `tfsdk:"domain"`
	Type     types.String `tfsdk:"type"`
	Name     types.String `tfsdk:"name"`
	Content  types.String `tfsdk:"content"`
	TTL      types.Int64  `tfsdk:"ttl"`
	RecordID types.String `tfsdk:"record_id"`
}

func New() resource.Resource {
	return &resourceDNSRecord{}
}

func (r *resourceDNSRecord) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *resourceDNSRecord) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				Required: true,
				PlanModifiers: []rplan.String{
					splan.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required: true,
				PlanModifiers: []rplan.String{
					splan.RequiresReplace(), // update is delete+create; treat type change as replace
				},
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"content": schema.StringAttribute{
				Required: true,
			},
			"ttl": schema.Int64Attribute{
				Required: true,
			},
			"record_id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *resourceDNSRecord) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.api = req.ProviderData.(*client.Client)
}

func (r *resourceDNSRecord) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := normalizeNoDotLower(plan.Domain.ValueString())
	name := normalizeNoDotLower(plan.Name.ValueString())

	prefix, err := computePrefix(domain, name)
	if err != nil {
		resp.Diagnostics.AddError("Invalid record name for domain", err.Error())
		return
	}

	// Read-before-create to reduce accidental duplicates.
	dom, err := r.getDomain(ctx, domain)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read domain before create", err.Error())
		return
	}
	if existing := findRecordByFields(dom.Records, plan.Type.ValueString(), name, plan.Content.ValueString(), plan.TTL.ValueInt64()); existing != nil {
		plan.RecordID = types.StringValue(existing.ID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	updated, err := r.api.UpdateDomain(ctx, domain, client.UpdateDomainRequest{
		Records: []client.UpdateRecord{
			{
				Type:    plan.Type.ValueString(),
				TTL:     plan.TTL.ValueInt64(),
				Prefix:  prefix,
				Name:    name,
				Content: plan.Content.ValueString(),
				Action:  "CREATE",
			},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create DNS record", err.Error())
		return
	}

	created := findRecordByFields(updated.Records, plan.Type.ValueString(), name, plan.Content.ValueString(), plan.TTL.ValueInt64())
	if created == nil || created.ID == "" {
		resp.Diagnostics.AddError(
			"Unable to correlate created record",
			"Create succeeded but provider could not find created record in response. If duplicates exist, delete them manually and re-apply.",
		)
		return
	}

	plan.Domain = types.StringValue(domain)
	plan.Name = types.StringValue(name)
	plan.RecordID = types.StringValue(created.ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceDNSRecord) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := normalizeNoDotLower(state.Domain.ValueString())

	domainsResp, err := r.api.GetDomains(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read domains", err.Error())
		return
	}

	dom := findDomain(domainsResp.Docs, domain)
	if dom == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	rid := state.RecordID.ValueString()
	if rid == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	rec := findRecordByID(dom.Records, rid)
	if rec == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// refresh state from API canonical values
	state.Domain = types.StringValue(domain)
	state.Type = types.StringValue(rec.Type)
	state.Name = types.StringValue(normalizeNoDotLower(rec.Name))
	state.Content = types.StringValue(rec.Content)
	state.TTL = types.Int64Value(rec.TTL)
	state.RecordID = types.StringValue(rec.ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceDNSRecord) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnsRecordModel
	var state dnsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := normalizeNoDotLower(plan.Domain.ValueString())
	newName := normalizeNoDotLower(plan.Name.ValueString())

	// If record_id exists, delete by id then create new desired.
	oldID := state.RecordID.ValueString()
	if oldID != "" {
		_, err := r.api.UpdateDomain(ctx, domain, client.UpdateDomainRequest{
			Records: []client.UpdateRecord{
				{ID: oldID, Action: "DELETE"},
			},
		})
		if err != nil {
			resp.Diagnostics.AddError("Unable to delete DNS record during update", err.Error())
			return
		}
	}

	prefix, err := computePrefix(domain, newName)
	if err != nil {
		resp.Diagnostics.AddError("Invalid record name for domain", err.Error())
		return
	}

	updated, err := r.api.UpdateDomain(ctx, domain, client.UpdateDomainRequest{
		Records: []client.UpdateRecord{
			{
				Type:    plan.Type.ValueString(),
				TTL:     plan.TTL.ValueInt64(),
				Prefix:  prefix,
				Name:    newName,
				Content: plan.Content.ValueString(),
				Action:  "CREATE",
			},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create DNS record during update", err.Error())
		return
	}

	created := findRecordByFields(updated.Records, plan.Type.ValueString(), newName, plan.Content.ValueString(), plan.TTL.ValueInt64())
	if created == nil || created.ID == "" {
		resp.Diagnostics.AddError("Unable to correlate created record during update", "Create returned no match in records list.")
		return
	}

	plan.Domain = types.StringValue(domain)
	plan.Name = types.StringValue(newName)
	plan.RecordID = types.StringValue(created.ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceDNSRecord) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := normalizeNoDotLower(state.Domain.ValueString())
	rid := state.RecordID.ValueString()
	if rid == "" {
		return
	}

	_, err := r.api.UpdateDomain(ctx, domain, client.UpdateDomainRequest{
		Records: []client.UpdateRecord{
			{ID: rid, Action: "DELETE"},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete DNS record", err.Error())
		return
	}
}

func (r *resourceDNSRecord) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// ID format: domain/record_id
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import id", "Expected format: domain/record_id (e.g. agtc.am/c841...)")
		return
	}
	domain := normalizeNoDotLower(parts[0])
	rid := parts[1]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("record_id"), rid)...)
}

func (r *resourceDNSRecord) getDomain(ctx context.Context, domain string) (*client.Domain, error) {
	resp, err := r.api.GetDomains(ctx)
	if err != nil {
		return nil, err
	}
	d := findDomain(resp.Docs, domain)
	if d == nil {
		return nil, fmt.Errorf("domain not found: %s", domain)
	}
	return d, nil
}

func findDomain(docs []client.Domain, domain string) *client.Domain {
	for i := range docs {
		if normalizeNoDotLower(docs[i].Domain) == domain {
			return &docs[i]
		}
	}
	return nil
}

func findRecordByID(records []client.DNSRecord, id string) *client.DNSRecord {
	for i := range records {
		if records[i].ID == id {
			return &records[i]
		}
	}
	return nil
}

func findRecordByFields(records []client.DNSRecord, typ, name, content string, ttl int64) *client.DNSRecord {
	typ = strings.TrimSpace(typ)
	name = normalizeNoDotLower(name)
	for i := range records {
		if records[i].Type == typ &&
			normalizeNoDotLower(records[i].Name) == name &&
			records[i].Content == content &&
			records[i].TTL == ttl {
			return &records[i]
		}
	}
	return nil
}

func normalizeNoDotLower(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".")
	return strings.ToLower(s)
}

func computePrefix(domain, name string) (string, error) {
	domain = normalizeNoDotLower(domain)
	name = normalizeNoDotLower(name)

	if name == domain {
		return "@", nil
	}
	suffix := "." + domain
	if !strings.HasSuffix(name, suffix) {
		return "", fmt.Errorf("record name %q must be within domain %q", name, domain)
	}
	prefix := strings.TrimSuffix(name, suffix)
	if prefix == "" {
		return "@", nil
	}
	return prefix, nil
}
