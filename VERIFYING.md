# Verifying release artifacts

Every release on the [Releases page](https://github.com/directedbits/recur/releases) publishes:

- Binary tarballs (`recur-vX.Y.Z-<os>-<arch>.tar.gz`)
- A checksum file (`recur-vX.Y.Z-checksums.txt`)
- A cosign signature bundle (`recur-vX.Y.Z-checksums.txt.cosign.bundle`)

## Quick check — integrity only

Confirms the tarballs match what was published (catches network corruption,
mirror tampering). Does not prove the checksum file itself is authentic.

```sh
sha256sum -c recur-vX.Y.Z-checksums.txt
```

## Full check — authenticity

Confirms the checksum file was signed by the official release workflow in
this repository. Once the checksum file is trusted, the integrity check
above proves each binary.

Requires [cosign](https://github.com/sigstore/cosign).

```sh
cosign verify-blob \
  --certificate-identity-regexp '^https://github\.com/directedbits/recur/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --bundle recur-vX.Y.Z-checksums.txt.cosign.bundle \
  recur-vX.Y.Z-checksums.txt
```

A successful run prints `Verified OK` and proves:

1. The checksum file was signed by a GitHub Actions workflow under
   `directedbits/recur`.
2. The signature was recorded in the public
   [Rekor transparency log](https://search.sigstore.dev/), so any forged
   release would leave a verifiable record.

## How the signing works

We use [Sigstore cosign in keyless mode](https://docs.sigstore.dev/cosign/signing/overview/).
The release workflow exchanges its short-lived GitHub OIDC token for a
single-use signing certificate from Sigstore's Fulcio CA, signs the
checksums, and logs the signature in Rekor. No long-lived private key
exists, so there is nothing to rotate or to leak.
