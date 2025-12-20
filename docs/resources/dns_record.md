---
page_title: "nameam_dns_record"
description: "Manage a DNS record for a Name.am domain."
---

# nameam_dns_record

Manages a single DNS record for a Name.am domain.

## Example Usage

```hcl
resource "nameam_dns_record" "txt" {
  domain  = "example.am"
  type    = "TXT"
  name    = "tf.example.am"
  content = "hello"
  ttl     = 1
}
```

## Argument Reference

`domain` (Required)  
Domain name (for example `example.am`).

`type` (Required)  
DNS record type (for example `A`, `AAAA`, `CNAME`, `TXT`, `MX`).

`name` (Required)  
Record name as a fully-qualified domain name (FQDN), for example `www.example.am` or `example.am`.

`content` (Required)  
Record value.

`ttl` (Optional)  
Record TTL. If not set, the provider uses the API default behavior. For Name.am examples, TTL is commonly `1`.

## Attributes Reference

`record_id`  
The Name.am record identifier returned by the API.

## Import

Import by providing the domain and record id in the form:

```bash
terraform import nameam_dns_record.txt "example.am/<record_id>"
```

## Notes on Behavior

Create and delete operations are performed using `PUT /client/domains/{domain}` with `records[].action` set to `CREATE` or `DELETE`. The provider uses the record `id` returned by Name.am as the stable identity key.
