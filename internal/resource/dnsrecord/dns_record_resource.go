package dnsrecord

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	rplan "github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	splan "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
				Required:    true,
				Description: "Lowercase domain name without a trailing dot.",
				PlanModifiers: []rplan.String{
					splan.RequiresReplace(),
				},
				Validators: []validator.String{canonicalDNSNameValidator{label: "domain"}},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Uppercase DNS record type.",
				Validators:  []validator.String{uppercaseRecordTypeValidator{}},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Lowercase fully-qualified record name without a trailing dot.",
				Validators:  []validator.String{canonicalDNSNameValidator{label: "record name"}},
			},
			"content": schema.StringAttribute{
				Required:    true,
				Description: "Record content, passed to Name.am exactly.",
			},
			"ttl": schema.Int64Attribute{
				Required:    true,
				Description: "TTL value. Name.am documents no default, so it is required.",
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
	if existing := findRecordsByFields(domain, dom.Records, plan.Type.ValueString(), name, plan.Content.ValueString(), plan.TTL.ValueInt64()); len(existing) > 0 {
		resp.Diagnostics.AddError(
			"DNS record already exists",
			fmt.Sprintf("A matching record already exists in %s with id %s. Import it explicitly with `terraform import nameam_dns_record.<name> %s/%s`; the provider will not silently take ownership of unmanaged records.", domain, existing[0].ID, domain, existing[0].ID),
		)
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

	createdMatches := findRecordsByFields(domain, updated.Records, plan.Type.ValueString(), name, plan.Content.ValueString(), plan.TTL.ValueInt64())
	if len(createdMatches) != 1 || createdMatches[0].ID == "" {
		resp.Diagnostics.AddError(
			"Unable to correlate created record",
			fmt.Sprintf("Create succeeded but provider found %d matching records in the response; exactly one was expected. Inspect the zone before retrying.", len(createdMatches)),
		)
		return
	}
	created := createdMatches[0]

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

	dom, err := r.api.GetDomain(ctx, domain)
	if errors.Is(err, client.ErrDomainNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read domains", err.Error())
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
	state.Name = types.StringValue(normalizeRecordFQDN(domain, rec.Name))
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

	prefix, err := computePrefix(domain, newName)
	if err != nil {
		resp.Diagnostics.AddError("Invalid record name for domain", err.Error())
		return
	}

	current, err := r.getDomain(ctx, domain)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read domain before update", err.Error())
		return
	}

	oldID := state.RecordID.ValueString()
	if oldID == "" || findRecordByID(current.Records, oldID) == nil {
		resp.Diagnostics.AddError("Managed DNS record is missing", "The record disappeared before update. Refresh Terraform state and apply again.")
		return
	}
	for _, match := range findRecordsByFields(domain, current.Records, plan.Type.ValueString(), newName, plan.Content.ValueString(), plan.TTL.ValueInt64()) {
		if match.ID != oldID {
			resp.Diagnostics.AddError(
				"Updated DNS record would conflict with an unmanaged record",
				fmt.Sprintf("A matching desired record already exists with id %s. Import or remove that record before updating this resource.", match.ID),
			)
			return
		}
	}

	updateRequest := replacementRequest(oldID, plan.Type.ValueString(), newName, prefix, plan.Content.ValueString(), plan.TTL.ValueInt64())
	updated, err := r.api.UpdateDomain(ctx, domain, updateRequest)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create DNS record during update", err.Error())
		return
	}

	createdMatches := findRecordsByFields(domain, updated.Records, plan.Type.ValueString(), newName, plan.Content.ValueString(), plan.TTL.ValueInt64())
	if len(createdMatches) != 1 || createdMatches[0].ID == "" {
		resp.Diagnostics.AddError("Unable to correlate created record during update", fmt.Sprintf("Update returned %d matching records; exactly one was expected. Inspect the zone before retrying.", len(createdMatches)))
		return
	}
	created := createdMatches[0]

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
	domain, rid, err := parseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("record_id"), rid)...)
}

func parseImportID(id string) (string, string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("expected format: domain/record_id (e.g. example.am/c841...)")
	}
	domain := normalizeNoDotLower(parts[0])
	if !strings.Contains(domain, ".") {
		return "", "", fmt.Errorf("import domain %q must be a fully-qualified domain name", parts[0])
	}
	return domain, strings.TrimSpace(parts[1]), nil
}

func (r *resourceDNSRecord) getDomain(ctx context.Context, domain string) (*client.Domain, error) {
	return r.api.GetDomain(ctx, domain)
}

func findRecordByID(records []client.DNSRecord, id string) *client.DNSRecord {
	for i := range records {
		if records[i].ID == id {
			return &records[i]
		}
	}
	return nil
}

func findRecordsByFields(domain string, records []client.DNSRecord, typ, name, content string, ttl int64) []*client.DNSRecord {
	typ = strings.TrimSpace(typ)
	name = normalizeRecordFQDN(domain, name)
	matches := make([]*client.DNSRecord, 0, 1)
	for i := range records {
		if records[i].Type == typ &&
			normalizeRecordFQDN(domain, records[i].Name) == name &&
			records[i].Content == content &&
			records[i].TTL == ttl {
			matches = append(matches, &records[i])
		}
	}
	return matches
}

func replacementRequest(oldID, recordType, name, prefix, content string, ttl int64) client.UpdateDomainRequest {
	return client.UpdateDomainRequest{Records: []client.UpdateRecord{
		{ID: oldID, Action: "DELETE"},
		{Type: recordType, TTL: ttl, Prefix: prefix, Name: name, Content: content, Action: "CREATE"},
	}}
}

func normalizeNoDotLower(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".")
	return strings.ToLower(s)
}

func normalizeRecordFQDN(domain, name string) string {
	domain = normalizeNoDotLower(domain)
	name = normalizeNoDotLower(name)
	if name == "" || name == "@" || name == domain {
		return domain
	}
	if strings.HasSuffix(name, "."+domain) {
		return name
	}
	return name + "." + domain
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
