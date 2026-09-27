package domains

import (
	"testing"

	"github.com/rserag/terraform-provider-nameam/internal/client"
)

func TestDomainsToModel(t *testing.T) {
	t.Parallel()
	model := domainsToModel([]client.Domain{
		{
			Domain:  "Example.AM",
			DnsUsed: true,
			NameServers: []client.NameServer{
				{Hostname: "ns1.example.am"},
				{Hostname: "  "},
				{Hostname: "ns2.example.am"},
			},
		},
	})
	if len(model.Domains) != 1 {
		t.Fatalf("domains=%+v", model.Domains)
	}
	domain := model.Domains[0]
	if domain.Domain.ValueString() != "example.am" || !domain.DnsUsed.ValueBool() {
		t.Fatalf("domain=%+v", domain)
	}
	if len(domain.NameServers) != 2 || domain.NameServers[0].ValueString() != "ns1.example.am" || domain.NameServers[1].ValueString() != "ns2.example.am" {
		t.Fatalf("nameservers=%+v", domain.NameServers)
	}
}

func TestDomainsToModelKeepsEmptyListKnown(t *testing.T) {
	t.Parallel()
	model := domainsToModel(nil)
	if model.Domains == nil || len(model.Domains) != 0 {
		t.Fatalf("domains=%+v", model.Domains)
	}
}
