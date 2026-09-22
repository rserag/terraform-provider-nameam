---
page_title: "Name.am Provider Safety Design"
description: "Ownership, drift, mutation retry, and normalization design."
---

# Safety design

The provider uses the official [Name.am API documentation](https://docs.name.am/) as its only API contract.

## Resource ownership

`nameam_dns_record` owns exactly one API record identified by the stable record `id`. Creation never claims an existing matching record: it returns an import instruction instead. This ensures that destroying a newly declared Terraform resource cannot unexpectedly delete a record that Terraform did not create or explicitly import.

## Reads and drift

DNS records are read from the domain objects returned by `GET /client/domains`. The provider follows the documented `page`, `limit`, `hasNextPage`, and `nextPage` fields and matches the requested domain only after collecting every page. Refresh locates the record by API ID, so deletion or changes outside Terraform are visible as drift.

## Mutations and retries

Name.am mutates records through `PUT /client/domains/{domain}` and documents that `CREATE`, `UPDATE`, and `DELETE` actions may be mixed in one `records` array.

Record replacement is sent as one batch containing `DELETE` for the old ID and `CREATE` for the desired record. GET requests may be retried normally. A mutation is never blindly retried after a transient or ambiguous failure:

1. Read the complete domain state.
2. If every requested action is visible, treat the request as successful.
3. If no action is visible, retry within the configured limit.
4. If only part of the batch is visible, stop with an explicit partial-mutation diagnostic.

This cannot make an upstream batch atomic, but it prevents duplicate creation and repeated deletion caused by ambiguous transport outcomes.

## Canonical values

- Domains and record names are lowercase FQDNs without trailing dots.
- Record types are uppercase; no closed type allowlist is imposed because Name.am documents none.
- The API's apex `@`, an empty owner, and the zone name normalize to the configured zone FQDN.
- Record content remains exact. TXT quotes and escapes are not rewritten.
- TTL is required because the API documentation provides no default contract.
