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

`ttl` (Required)

Record TTL. The provider applies no default because the Name.am API documentation specifies none.

## Attributes Reference

`record_id`  
The Name.am record identifier returned by the API.

## Import

Import by providing the domain and record id in the form:

```bash
terraform import nameam_dns_record.txt "example.am/<record_id>"
```

## Notes on Behavior

Create and delete operations are performed using `PUT /client/domains/{domain}` with `records[].action` set to `CREATE` or `DELETE`. Updates send both actions in one request. The provider uses the record `id` returned by Name.am as the stable identity key.

An existing matching record is never silently adopted. Import it explicitly using its record ID. After a transient mutation failure, the provider reads the zone and retries only if none of the requested actions took effect; a partial result requires operator inspection.

Domain names, record names, and record types must already be canonical: lowercase FQDNs without trailing dots, and uppercase record types. Record content is passed through exactly, including TXT quoting and escaping.
