## Summary

<!-- What does this PR do and why? Prefer short explanations. -->

- [ ] Are new tests required?
- [ ] Does documentation need to be updated? (see Checklist below for what this covers)
- [ ] Contains breaking changes?
- [ ] Touches plugin contract, manifest format, or recurfile YAML schema?
- [ ] Changes daemon config keys or plugin options?
- [ ] Affects daemon startup/shutdown behavior?
- [ ] Changes CLI flags or command signatures?

## Changes

<!-- Bulleted list of what changed. -->

-

## Test plan

<!-- How can a reviewer verify this works? -->

- [ ] Unit tests pass (`task test`)
- [ ] Plugin tests pass (`task test:plugins`)
- [ ] E2E tests pass (`task test:e2e`) — if applicable

## Checklist

- [ ] Tests added or updated for new/changed behavior
- [ ] Spec in `requirements/` updated or added — write the spec
      alongside the code so reviewers can verify "code matches
      spec" rather than re-deriving intent. Skip only for changes
      that don't affect behavior (refactors, dep bumps, test-only
      fixes).
- [ ] User docs updated for user-observable changes (see
      [CONTRIBUTING § Documentation](.github/CONTRIBUTING.md#documentation)
      for triggers). **Some pages are synced**: plugin docs come from
      `plugins/<name>/README.md`, contributing from
      `.github/CONTRIBUTING.md`, the docker example from
      `examples/docker/README.md` — edit the source and run
      `task docs:sync`. Everything else: edit `docs/content/docs/`
      directly.
- [ ] Small, buildable commits — each compiles and passes tests, one
      logical change per commit, message explains *why*
- [ ] No breaking changes — or noted below with migration steps

## Breaking changes

<!-- If this is a breaking change, describe what breaks and how users should migrate. Otherwise delete this section. -->

None.
