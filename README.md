# terraform-provider-nameam

Terraform provider for **Name.am** that allows you to:
- List domains in your Name.am account
- Manage DNS records (A, CNAME, TXT, etc.) using Terraform

This provider is intentionally **minimal** and **strictly based on the official Name.am API**.
It is designed for correctness, idempotency, and safe DNS management.

---

## Features (v1)

### Supported
- Provider authentication using JWT bearer token
- Data source to list domains
- Resource to create, update (recreate), delete, and import DNS records

### Explicitly NOT supported
- Domain transfers
- Billing
- Renewals
- WHOIS privacy
- Contacts
- DNSSEC
- Nameserver changes
- Any undocumented API behavior

---

## Requirements

- Terraform >= 1.6
- Go >= 1.22 (for development)
- Valid Name.am JWT token

---

## Provider Configuration

```hcl
provider "nameam" {
  token = var.nameam_token
}
```

### Arguments

| Name  | Type   | Required | Description |
|------|--------|----------|-------------|
| token | string | yes | Name.am JWT bearer token |

---

## Data Sources

### `nameam_domains`

Lists all domains available in the account.

```hcl
data "nameam_domains" "this" {}

output "domains" {
  value = data.nameam_domains.this.domains[*].domain
}
```

---

## Resources

### `nameam_dns_record`

Manages a single DNS record.

#### Example: TXT record

```hcl
resource "nameam_dns_record" "txt" {
  domain  = "example.am"
  type    = "TXT"
  name    = "test.example.am"
  content = "hello"
  ttl     = 1
}
```

#### Example: Apex record

```hcl
resource "nameam_dns_record" "apex_txt" {
  domain  = "example.am"
  type    = "TXT"
  name    = "example.am"
  content = "root"
  ttl     = 1
}
```

### Arguments

| Name | Type | Required | Description |
|-----|------|----------|-------------|
| domain | string | yes | Domain name (e.g. `example.am`) |
| type | string | yes | DNS record type |
| name | string | yes | Fully-qualified DNS name |
| content | string | yes | Record value |
| ttl | number | yes | TTL value |

### Computed Attributes

| Name | Description |
|-----|-------------|
| record_id | Internal Name.am record ID |

---

## Import

Records can be imported using:

```bash
terraform import nameam_dns_record.example example.am/<record_id>
```

After import:

```bash
terraform plan
```

Should show **no changes**.

---

## Update Semantics

The Name.am API does not support in-place DNS record updates.

Terraform updates are implemented as:
1. DELETE existing record by ID
2. CREATE new record
3. Store new record ID

This behavior is intentional and documented.

---

## Local Development

### Build provider
```bash
make build
```

### Local testing with dev override
Use `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "rserag/nameam" = "/absolute/path/to/terraform-provider-nameam/bin"
  }
  direct {
    exclude = ["rserag/nameam"]
  }
}
```

---

## Limitations

- Domain listing endpoint includes pagination fields, but request-side pagination is not documented.
- Provider currently fetches all domains in a single request.
- Pagination support will be added only if officially documented.

---

## License

MIT
