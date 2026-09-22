# Authoritative DNS zone resource design

Status: design complete; implementation intentionally deferred until the existing single-record lifecycle passes live acceptance testing.

## API contract

The design uses only behavior documented by Name.am:

- `GET /client/domains` returns paginated domain objects and is the read path.
- Domain objects include the DNS record collection.
- `PUT /client/domains/{domain}` accepts a `records` array whose entries use `CREATE`, `UPDATE`, or `DELETE` actions.
- Record IDs returned by the API are the only stable remote identifiers used for deletion or correlation.

The documentation does not define a dedicated zone endpoint, a transaction guarantee for mixed actions, a TTL default, or a canonicalization contract for record content. The provider must not infer any of those behaviors.

## Proposed Terraform shape

```hcl
resource "nameam_dns_zone" "example" {
  domain = "example.am"

  records = [
    {
      type    = "A"
      name    = "www.example.am"
      content = "192.0.2.10"
      ttl     = 300
    },
  ]
}
```

`domain` is immutable. `records` is an authoritative set: every manageable DNS record returned for the domain must appear in configuration. A plan removes remote records omitted from the set and adds configured records absent remotely.

Each record uses the same canonical inputs as `nameam_dns_record`: uppercase `type`, lowercase fully-qualified `name` without a trailing dot, exact `content`, and explicit `ttl`. API IDs remain computed internal state and do not participate in user configuration.

## Identity and reconciliation

The desired identity key is the tuple `(type, normalized name, exact content, ttl)`. This is sufficient for set comparison but may not be unique in the remote API. The implementation must retain a multimap from that key to API IDs so duplicate records are reconciled by count and deletion always targets a concrete ID.

Refresh reads the complete paginated domain list and replaces Terraform state with the observed record multiset. Planning compares desired and observed multiplicities:

- surplus remote instances become `DELETE` actions by ID;
- missing desired instances become `CREATE` actions;
- unchanged instances keep their IDs;
- changes are expressed as delete-plus-create unless live testing proves documented `UPDATE` behavior is reliable.

All changes should be submitted in one mixed `records` batch. The client's existing ambiguous-failure reconciliation must be extended to compare multisets, including duplicates, before zone mutations are enabled.

## Ownership and deletion safety

This resource is deliberately destructive: it owns the complete manageable record set for one domain. Import uses the domain name alone and immediately adopts every returned record:

```shell
terraform import nameam_dns_zone.example example.am
```

Import should populate state but configuration must still declare every record before the next apply. Documentation and the import diagnostic must tell users to run a refresh-only plan and copy the observed set into configuration before making changes.

Destroy semantics should remove all records currently tracked by the resource, using their IDs. It must not delete a record that appeared after the last refresh without first detecting and planning that drift. Because a zone destroy can remove essential mail or website records, the schema should expose an explicit `allow_destroy = true` guard, defaulting to false. This is a provider safety control, not an API field.

## Coexistence rule

`nameam_dns_zone` and `nameam_dns_record` must never manage the same domain. Terraform Plugin Framework cannot reliably discover every sibling resource across modules or states, so schema validation alone cannot enforce global exclusivity.

The practical v1 rule is therefore:

1. Publish the two ownership models as mutually exclusive for a domain.
2. Add an opt-in provider process lock keyed by normalized domain so concurrent operations in one provider process fail clearly.
3. Add remote-state ownership markers only if Name.am later documents metadata suitable for that purpose; do not create synthetic DNS records as locks.
4. Treat cross-state mixing as unsupported and warn that the authoritative resource will remove individually managed records.

## Required tests before implementation ships

Offline tests:

- deterministic multiset diff with duplicate records;
- apex normalization and exact TXT content preservation;
- import-state population;
- destroy guard behavior;
- one mixed mutation request per reconciliation;
- ambiguous full, partial, and unapplied batch outcomes;
- pagination across the target domain and stable no-op plans.

Live acceptance tests on a disposable domain:

- import an existing non-empty zone and obtain a no-op plan after configuration is mirrored;
- create, update, delete, duplicate, and reorder records;
- verify a second plan is empty after every apply;
- introduce out-of-band drift and verify it is planned;
- simulate coexistence with `nameam_dns_record` and confirm the documented failure/warning;
- confirm guarded destroy and inspect the final zone manually.

## Decision

Do not implement or register `nameam_dns_zone` before the current `nameam_dns_record` live acceptance test succeeds. Both resources depend on the same mutation endpoint and response correlation behavior; shipping a broader authoritative resource before validating that foundation would multiply the blast radius. Once credentials and a disposable domain are available, validate the single-record lifecycle first, then implement this design behind its own acceptance suite.
