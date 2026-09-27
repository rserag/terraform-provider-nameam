package resource_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	tfprotov6 "github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	nameamprovider "github.com/rserag/terraform-provider-nameam/internal/provider"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	// NOTE: In this framework version, NewProtocol6WithError expects a provider instance.
	// Our provider.New("test") returns a factory, so we call it: ...New("test")()
	"nameam": providerserver.NewProtocol6WithError(nameamprovider.New("test")()),
}

func TestAccDNSRecord_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}

	domain := os.Getenv("NAMEAM_DOMAIN")
	if os.Getenv("NAMEAM_TOKEN") == "" || domain == "" {
		t.Fatalf("NAMEAM_TOKEN and NAMEAM_DOMAIN must be set")
	}
	recordName := fmt.Sprintf("tfacc-%d.%s", time.Now().UnixNano(), domain)

	createConfig := testAccDNSRecordConfig(domain, recordName, "hello")
	updateConfig := testAccDNSRecordConfig(domain, recordName, "updated")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: createConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("nameam_dns_record.txt", "record_id"),
				),
			},
			{
				Config: updateConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nameam_dns_record.txt", "content", "updated"),
					resource.TestCheckResourceAttrSet("nameam_dns_record.txt", "record_id"),
				),
			},
			{
				ResourceName:      "nameam_dns_record.txt",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					resourceState := state.RootModule().Resources["nameam_dns_record.txt"]
					if resourceState == nil || resourceState.Primary == nil {
						return "", fmt.Errorf("nameam_dns_record.txt not found in state")
					}
					return domain + "/" + resourceState.Primary.Attributes["record_id"], nil
				},
			},
		},
	})
}

func testAccDNSRecordConfig(domain, recordName, content string) string {
	return fmt.Sprintf(`
provider "nameam" {}

resource "nameam_dns_record" "txt" {
  domain  = %q
  type    = "TXT"
  name    = %q
  content = %q
  ttl     = 1
}
`, domain, recordName, content)
}
