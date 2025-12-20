package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func TestAccProviderFactories(t *testing.T) map[string]func() (any, error) {
	t.Helper()

	return map[string]func() (any, error){
		"nameam": providerserver.NewProtocol6WithError(New()),
	}
}
