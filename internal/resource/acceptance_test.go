package dnsrecord

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/rserag/terraform-provider-nameam/internal/provider"
)

func TestAccDNSRecord_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}

	token := os.Getenv("NAMEAM_TOKEN")
	domain := os.Getenv("NAMEAM_DOMAIN")
	if token == "" || domain == "" {
		t.Fatalf("NAMEAM_TOKEN and NAMEAM_DOMAIN must be set")
	}

	cfg := fmt.Sprintf(`
provider "nameam" {
  token = %q
}

resource "nameam_dns_record" "txt" {
  domain  = %q
  type    = "TXT"
  name    = "tfacc.%s"
  content = "hello"
  ttl     = 1
}
`, token, domain, domain)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: provider.TestAccProviderFactories(t),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("nameam_dns_record.txt", "record_id"),
				),
			},
		},
	})
}
