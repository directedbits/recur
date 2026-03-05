# Homebrew tap packaging

Source of truth for the Homebrew formula published at
`directedbits/homebrew-tap`. The release workflow renders
`recur.rb.template` into the tap repo on every tagged release.

## Files

- `recur.rb.template` — Formula source with `${VERSION}` and
  `${SHA256_<PLATFORM>}` placeholders. Edited here, never edited in the
  tap repo directly.
- `update-tap.job.yml` — Job to append to `.github/workflows/release.yml`.

## One-time setup

1. **Create the tap repo.** GitHub: new repo `directedbits/homebrew-tap`,
   empty, public. The `homebrew-` prefix is mandatory — that's how
   `brew tap directedbits/tap` resolves.

2. **Create the access token.** GitHub Settings → Developer settings →
   Fine-grained PATs. Scope: `directedbits/homebrew-tap` only.
   Permission: `Contents: Read and write`.
   Expiry: longest you're willing to manage (1 year max).

3. **Store the token.** This repo's Settings → Secrets and variables →
   Actions → New repository secret: `HOMEBREW_TAP_TOKEN` = the PAT.

4. **Wire the job.** Paste `update-tap.job.yml` at the bottom of
   `.github/workflows/release.yml` (under `jobs:`). The leading
   indentation is correct as-is.

5. **First release.** Tag a release (`git tag v0.1.0 && git push --tags`).
   The build matrix uploads tarballs, then `update-homebrew-tap` downloads
   them, computes SHA256s, renders the template, and pushes the result
   to the tap repo. The first push creates `Formula/recur.rb`.

## User install (after first release)

```sh
brew tap directedbits/tap
brew install recur
brew services start recur
```

## Editing the formula

Edit `recur.rb.template` in this repo, commit, tag a new release. The
workflow re-renders. Never hand-edit `Formula/recur.rb` in the tap repo —
the next release will overwrite it.

## Failure modes

- **403 on push to tap:** PAT expired or wrong scope.
- **`envsubst: command not found`:** the action's ubuntu-latest image
  includes gettext-base; if a future image drops it, swap to a `sed`-
  based templater.
- **`sha256 mismatch` on user install:** someone deleted and re-uploaded
  a release asset. Don't do that — tag a new patch release instead.
- **Stale formula after rebuilds:** the workflow re-runs on every push
  to a tag. Don't move a tag — that would silently change checksums.
