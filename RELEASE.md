# Release plan (Terraform Registry)

This repository is set up to publish signed releases to GitHub and then be ingested by the Terraform Registry.

## One-time setup

1) Create a GPG key (RSA, 4096-bit) and add the **public key** to Terraform Registry signing keys.

2) Add the following GitHub Actions secrets in this repository:

- `GPG_PRIVATE_KEY` – ASCII-armored private key export
- `GPG_PASSPHRASE` – passphrase for the private key
- `GPG_FINGERPRINT` – full fingerprint (recommended)

Export commands (local):

```bash
gpg --list-secret-keys --keyid-format=long
gpg --armor --export-secret-keys <YOUR_KEY_ID_OR_FPR>
```

3) Ensure provider identity consistency:

- Provider server address in `main.go`:
  `registry.terraform.io/rserag/nameam`
- Terraform usage:
  `source = "rserag/nameam"`

4) Publish the provider **once** in the Terraform Registry UI:

Terraform Registry → Publish → Provider → select this GitHub repository.

## Release steps

1) Create and push a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

2) GitHub Actions will automatically:

- build provider binaries for supported OS/architectures
- create versioned ZIP archives
- generate `SHA256SUMS`
- sign `SHA256SUMS` with your GPG key
- create or update the GitHub Release

3) Terraform Registry detects the new GitHub Release and ingests the provider version.

Users can then install the provider with:

```hcl
terraform {
  required_providers {
    nameam = {
      source  = "rserag/nameam"
      version = "0.1.0"
    }
  }
}
```

## Supported platforms (v1)

- darwin_amd64
- darwin_arm64
- linux_amd64
- linux_arm64
- windows_amd64

## Troubleshooting

### Signature verification failure
If Terraform Registry reports a checksum signature error:

- Verify the public GPG key uploaded to Terraform Registry matches the private key used in CI
- Ensure `GPG_FINGERPRINT` matches the imported key
- Confirm both `*_SHA256SUMS` and `*_SHA256SUMS.sig` are present in the GitHub Release

### Bad or cached release
If Registry ingests a broken release:

- Delete the GitHub Release
- Bump the version (e.g. `v0.1.1`)
- Push a new tag

---

This process follows HashiCorp’s official Terraform provider publishing requirements.
