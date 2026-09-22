package dnsrecord

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/rserag/terraform-provider-nameam/internal/client"
)

func TestComputePrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		domain  string
		record  string
		want    string
		wantErr bool
	}{
		{name: "apex", domain: "example.am", record: "example.am", want: "@"},
		{name: "subdomain", domain: "example.am", record: "www.example.am", want: "www"},
		{name: "nested", domain: "example.am", record: "a.b.example.am", want: "a.b"},
		{name: "outside zone", domain: "example.am", record: "other.am", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := computePrefix(tt.domain, tt.record)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("prefix=%q err=%v", got, err)
			}
		})
	}
}

func TestNormalizeRecordFQDN(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"@":               "example.am",
		"example.am.":     "example.am",
		"www":             "www.example.am",
		"WWW.EXAMPLE.AM.": "www.example.am",
	}
	for input, want := range tests {
		if got := normalizeRecordFQDN("example.am", input); got != want {
			t.Errorf("normalizeRecordFQDN(%q)=%q want=%q", input, got, want)
		}
	}
}

func TestFindRecordsByFieldsNormalizesApex(t *testing.T) {
	t.Parallel()
	records := []client.DNSRecord{
		{ID: "one", Type: "A", Name: "@", Content: "192.0.2.1", TTL: 300},
		{ID: "two", Type: "A", Name: "example.am", Content: "192.0.2.1", TTL: 300},
	}
	matches := findRecordsByFields("example.am", records, "A", "example.am", "192.0.2.1", 300)
	if len(matches) != 2 {
		t.Fatalf("matches=%+v", matches)
	}
}

func TestReplacementRequestUsesSingleMixedBatch(t *testing.T) {
	t.Parallel()
	request := replacementRequest("old", "TXT", "txt.example.am", "txt", "new", 300)
	if len(request.Records) != 2 || request.Records[0].Action != "DELETE" || request.Records[0].ID != "old" || request.Records[1].Action != "CREATE" {
		t.Fatalf("request=%+v", request)
	}
}

func TestFindRecordByID(t *testing.T) {
	t.Parallel()
	records := []client.DNSRecord{{ID: "one"}, {ID: "two"}}
	if got := findRecordByID(records, "two"); got == nil || got.ID != "two" {
		t.Fatalf("record=%+v", got)
	}
	if got := findRecordByID(records, "missing"); got != nil {
		t.Fatalf("record=%+v", got)
	}
}

func TestParseImportID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id         string
		wantDomain string
		wantRecord string
		wantErr    bool
	}{
		{id: "Example.AM./ record-1 ", wantDomain: "example.am", wantRecord: "record-1"},
		{id: "example.am", wantErr: true},
		{id: "/record-1", wantErr: true},
		{id: "example.am/", wantErr: true},
		{id: "localhost/record-1", wantErr: true},
		{id: "example.am/record-1/extra", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			domain, record, err := parseImportID(tt.id)
			if (err != nil) != tt.wantErr || domain != tt.wantDomain || record != tt.wantRecord {
				t.Fatalf("domain=%q record=%q error=%v", domain, record, err)
			}
		})
	}
}

func TestCanonicalDNSNameValidator(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value   string
		wantErr bool
	}{
		{value: "example.am"},
		{value: "www.example.am"},
		{value: "Example.am", wantErr: true},
		{value: "example.am.", wantErr: true},
		{value: "localhost", wantErr: true},
	}
	for _, tt := range tests {
		var response validator.StringResponse
		canonicalDNSNameValidator{label: "DNS name"}.ValidateString(context.Background(), validator.StringRequest{
			Path: path.Root("name"), ConfigValue: types.StringValue(tt.value),
		}, &response)
		if response.Diagnostics.HasError() != tt.wantErr {
			t.Errorf("value=%q diagnostics=%v", tt.value, response.Diagnostics)
		}
	}
}

func TestUppercaseRecordTypeValidator(t *testing.T) {
	t.Parallel()
	for value, wantErr := range map[string]bool{"A": false, "TXT": false, "a": true, " TXT": true, "": true} {
		var response validator.StringResponse
		uppercaseRecordTypeValidator{}.ValidateString(context.Background(), validator.StringRequest{
			Path: path.Root("type"), ConfigValue: types.StringValue(value),
		}, &response)
		if response.Diagnostics.HasError() != wantErr {
			t.Errorf("value=%q diagnostics=%v", value, response.Diagnostics)
		}
	}
}
