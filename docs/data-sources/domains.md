---
page_title: "nameam_domains"
description: "List domains for the authenticated Name.am account."
---

# nameam_domains

Retrieves the list of domains associated with the authenticated Name.am account.

## Example Usage

```hcl
data "nameam_domains" "all" {}

output "domains" {
  value = data.nameam_domains.all.domains
}
```

## Attributes Reference

`domains`  
List of domain names.

## Notes

The API response includes additional metadata for each domain. This data source exposes only fields required for DNS workflows.
