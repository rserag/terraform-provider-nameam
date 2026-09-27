---
page_title: "Name.am Provider"
description: "Manage Name.am domains and DNS records."
---

# Name.am Provider

This provider manages DNS records for domains in Name.am via the Name.am REST API.

## Authentication

The provider authenticates using a Name.am API token. Set `NAMEAM_TOKEN` in the environment or configure the sensitive `token` argument.

```hcl
provider "nameam" {}
```

## Configuration Reference

`token` (Optional)

Name.am API token. Defaults to the `NAMEAM_TOKEN` environment variable.

`base_url` (Optional)  
Override the API base URL. Default is `https://api.name.am`.

`timeout` (Optional)  
HTTP client timeout as a Go duration string (for example `30s`). Default is `30s`.

`retries` (Optional)  
Number of retries for transient errors (429/5xx/timeouts). Default is `3`; set to `0` to disable retries. DNS mutations are reconciled with a read before any retry.

`retry_backoff` (Optional)  
Base retry backoff as a Go duration string (for example `1s`). Exponential backoff is applied. Default is `1s`.

## DNS ownership

Creating a resource fails if an identical record already exists. Import the existing record explicitly to transfer ownership to Terraform. Updates are sent as one mixed `DELETE`/`CREATE` batch, and ambiguous transient failures are reconciled before retry.

The provider relies on the domain listing endpoint to read DNS records and follows all documented pagination pages.
