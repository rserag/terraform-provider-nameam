terraform {
  required_version = ">= 1.6.0"

  required_providers {
    nameam = {
      source  = "rserag/nameam"
      version = "~> 0.1"
    }
  }
}

# Reads the API token from NAMEAM_TOKEN.
provider "nameam" {}

data "nameam_domains" "this" {}

output "domains" {
  value = data.nameam_domains.this.domains[*].domain
}

resource "nameam_dns_record" "txt" {
  domain  = "example.am"
  type    = "TXT"
  name    = "tf-example.example.am"
  content = "hello"
  ttl     = 1
}
