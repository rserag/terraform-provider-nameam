---
page_title: "Name.am Provider"
description: "Manage Name.am domains and DNS records."
---

# Name.am Provider

This provider manages DNS records for domains in Name.am via the Name.am REST API.

## Authentication

The provider authenticates using a JWT bearer token.

```hcl
provider "nameam" {
  token = var.nameam_token
}
```

## Configuration Reference

`token` (Required)
JWT bearer token for Name.am API.

`base_url` (Optional)
Override the API base URL. Default is `https://api.name.am`.

`timeout` (Optional)
HTTP client timeout as a Go duration string (for example `30s`). Default is `30s`.

`retries` (Optional)
Number of retries for transient errors (429/5xx/timeouts). Default is `3`.

`retry_backoff` (Optional)
Base retry backoff as a Go duration string (for example `1s`). Exponential backoff is applied. Default is `1s`.

`rate_limit` (Optional)
Maximum number of API requests per second. Default is `10`. Set to `0` to disable rate limiting. This helps prevent overwhelming the API when creating many DNS records concurrently.

## Known Limitations

The Name.am API surface used by this provider currently relies on the domain listing endpoint to read DNS records. Pagination is not implemented yet (TODO in code), consistent with available documentation and current implementation scope.
