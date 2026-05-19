<!--
Thanks for the PR! Keep the description tight — what changed and why,
not how. Reviewers can read the diff.

If the diff is >400 lines this PR is gated by `.github/workflows/pr-size.yml`.
Either split it, or apply the `size:exception` label with a one-line
justification in the description ("bundle is coherent: …").
-->

## Summary

<!-- 1-3 bullet points: what changed and what it unblocks. -->

## Test plan

<!-- A markdown checklist of how this is verified. Be specific.
For changes that touch the cloud deploy, include a curl-line
that should pass after the merge. -->

- [ ] `go test -race github.com/alcandev/korva/...` green
- [ ] `golangci-lint` 0 issues on touched files
- [ ] New behavior covered by tests (or rationale why not)
- [ ] Docs updated if user-visible behavior changed

## Out of scope / follow-ups

<!-- List anything you noticed but deliberately did NOT touch in this PR,
with a one-line reason. Helps the reviewer scope feedback and gives
future-you a punch list. -->
