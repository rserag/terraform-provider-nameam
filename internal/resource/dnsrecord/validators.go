package dnsrecord

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type canonicalDNSNameValidator struct {
	label string
}

func (v canonicalDNSNameValidator) Description(context.Context) string {
	return fmt.Sprintf("%s must be lowercase, fully qualified, and have no trailing dot", v.label)
}

func (v canonicalDNSNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v canonicalDNSNameValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if value == "" || !strings.Contains(value, ".") || value != normalizeNoDotLower(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid canonical DNS name",
			fmt.Sprintf("%s must be lowercase, fully qualified, contain no surrounding whitespace, and have no trailing dot.", v.label),
		)
	}
}

type uppercaseRecordTypeValidator struct{}

func (uppercaseRecordTypeValidator) Description(context.Context) string {
	return "record type must be non-empty uppercase text without surrounding whitespace"
}

func (v uppercaseRecordTypeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (uppercaseRecordTypeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if value == "" || value != strings.TrimSpace(value) || value != strings.ToUpper(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid DNS record type",
			"Record type must be non-empty uppercase text without surrounding whitespace. Name.am does not document a closed list of supported types.",
		)
	}
}
