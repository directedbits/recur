# Releasing recur

Maintainer playbook for cutting a release and validating the pipeline.

## One-time setup

1. **Push to GitHub.** The repo must live at `github.com/directedbits/recur`
   for the release workflow's identity claims and asset URLs to line up:
   ```sh
   git remote add github git@github.com:directedbits/recur.git
   git push github main
   ```

2. **Create the Homebrew tap repo** `directedbits/homebrew-tap` (empty,
   public). The `homebrew-` prefix is mandatory.

3. **Mint a fine-grained PAT** scoped to `directedbits/homebrew-tap` only,
   permission `Contents: Read and write`. Expiry: one year max.

4. **Store the PAT** as the secret `HOMEBREW_TAP_TOKEN` in
   `directedbits/recur` → Settings → Secrets and variables → Actions.

5. **Enable private vulnerability reporting** in the recur repo:
   Settings → Code security → "Private vulnerability reporting".

6. **Append the tap-update job** from `packaging/homebrew/update-tap.job.yml`
   to the bottom of `.github/workflows/release.yml` (under `jobs:`).

## Smoke-testing the pipeline

Goal: prove every step of `release.yml` works end-to-end before tagging
v0.1.0. Use a pre-release tag (`v0.0.0-rc<N>`) so the result is clearly
flagged as a prerelease and burns no real version numbers.

```sh
git tag v0.0.0-rc1
git push origin v0.0.0-rc1
```

Watch the workflow run in GitHub Actions. When green, verify:

### 1. Release page assets

`github.com/directedbits/recur/releases/tag/v0.0.0-rc1` should list:

- 8 platform tarballs (linux amd64/arm64/armv7/386, darwin amd64/arm64,
  windows amd64/arm64)
- `recur-v0.0.0-rc1-source.tar.gz`
- `recur-v0.0.0-rc1-checksums.txt`
- `recur-v0.0.0-rc1-checksums.txt.cosign.bundle`

### 2. User-side verification works

```sh
TAG=v0.0.0-rc1
base="https://github.com/directedbits/recur/releases/download/$TAG"
curl -OL "$base/recur-$TAG-checksums.txt"
curl -OL "$base/recur-$TAG-checksums.txt.cosign.bundle"

cosign verify-blob \
  --certificate-identity-regexp '^https://github\.com/directedbits/recur/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --bundle "recur-$TAG-checksums.txt.cosign.bundle" \
  "recur-$TAG-checksums.txt"
# expect: Verified OK
```

Then download any one tarball and confirm the integrity check passes:

```sh
curl -OL "$base/recur-$TAG-linux-amd64.tar.gz"
sha256sum -c "recur-$TAG-checksums.txt" --ignore-missing
# expect: recur-v0.0.0-rc1-linux-amd64.tar.gz: OK
```

### 3. Homebrew tap got updated

Visit `github.com/directedbits/homebrew-tap`. There should be a new commit
"recur 0.0.0-rc1" creating `Formula/recur.rb` with the rc1 version and
sha256s populated. Spot-check that the formula's `version` and `sha256`
values match the checksums file.

### 4. (Optional) Install from the tap

```sh
brew tap directedbits/tap
brew install recur
recur version          # should print v0.0.0-rc1
brew services start recur
recur status           # should show the daemon running
brew services stop recur
brew uninstall recur
```

## Iterating on failures

If anything fails, fix on `main`, then bump the rc:

```sh
# delete the failed prerelease (preserves transparency log entry but
# removes the broken assets from the release page)
gh release delete v0.0.0-rc1 --yes
git push --delete origin v0.0.0-rc1

git tag v0.0.0-rc2
git push origin v0.0.0-rc2
```

## Common failure modes

| Symptom | Cause | Fix |
|---|---|---|
| `cosign sign-blob` fails with "OIDC ID token request failed" | `id-token: write` missing from release job | Restore the job-level `permissions:` block in `release.yml` |
| tap-update step fails with 403 | `HOMEBREW_TAP_TOKEN` missing, expired, or wrong scope | Re-mint a fine-grained PAT scoped to `directedbits/homebrew-tap` only |
| tap update succeeds but pushes empty diff | `envsubst` not on runner | Confirm `gettext-base` is installed (currently default on `ubuntu-latest`); fall back to a sed templater if removed |
| `cosign verify-blob` fails "no matching signatures" | identity regex in `VERIFYING.md` doesn't match the workflow URL | Compare against the URL printed by the cosign step in the workflow log |
| `brew install` reports sha256 mismatch | a release asset was re-uploaded after the tap update ran | Don't re-upload assets — tag a new patch release |

## Doing the real release

Once the smoke test is green and the tap is correct:

1. Delete all rc tags and prereleases:
   ```sh
   for tag in $(git tag --list 'v0.0.0-rc*'); do
     gh release delete "$tag" --yes 2>/dev/null
     git push --delete origin "$tag"
   done
   ```
2. Write release notes (or rely on `generate_release_notes: true` to
   build them from PR titles).
3. Tag and push:
   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```
4. Re-run the verification steps above with the real tag.
5. Announce.

## Rolling back

A GitHub release can be deleted; the underlying tag can too. However,
the cosign signature is recorded in the public
[Rekor transparency log](https://search.sigstore.dev/) and cannot be
revoked — anyone who downloaded during the window the bad release was
live will still see a valid signature.

If a release is actively dangerous (vulnerability, malware, broken in a
data-losing way), the only remedy is to:

1. Delete the bad release (`gh release delete <tag>`).
2. Publish a fixed patch release immediately.
3. File a SECURITY ADVISORY via GitHub's private reporting flow.
4. Don't re-tag — move forward with `vX.Y.Z+1`.
