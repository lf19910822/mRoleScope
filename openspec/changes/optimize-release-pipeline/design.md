# Design

## Context

The release orchestrator currently downloads `bootstrap.sh` and then downloads
the same reviewed deployment assets sequentially on every release. The CVM
stores the active release under a root-owned directory but has no identity for
the installed deployment-boundary asset set. The existing root helper already
owns image pulls, migrations, health checks, rollback, and cleanup.

## Goals / Non-Goals

**Goals:**

- Make application-only releases avoid all unchanged deployment-asset network
  requests.
- Keep the existing bootstrap and restricted deployment helper as the only
  installation and activation boundaries.
- Make release timing observable without exposing secrets.

**Non-Goals:**

- Do not change image tags, digest validation, Compose networks, migrations,
  runtime credentials, or public exposure.
- Do not parallelize or remove quality checks in this change.
- Do not introduce a registry, mutable cache tag, or CVM source checkout.

## Decisions

### Fingerprint the reviewed asset set locally

The release script hashes every regular file under
`deploy/remote-development`, using sorted relative paths and file contents, and
then hashes the resulting list. The full clean `main` revision remains the
source identity for bootstrap URLs; the asset fingerprint only answers whether
the installed deployment boundary needs refresh.

### Store the fingerprint as a public root-owned fact

Bootstrap writes the validated fingerprint to a fixed root-owned,
non-secret file under `/opt/mrolescope`. The release command reads only this
safe fingerprint over SSH. A missing or mismatched file is a cache miss and
executes the existing staged-download, checksum, shell-check, and installation
flow.

### Keep manual bootstrap compatible

The third fingerprint argument is optional for existing manual initialization
instructions. The autonomous release path always supplies it; manual two-
argument bootstrap continues to install assets but does not claim an optimized
cache hit.

### Add timing at the orchestration boundary

The local script records monotonic shell timestamps around its major phases and
prints only phase names and milliseconds. It does not alter the remote helper's
authorization or print command arguments containing runtime configuration.

## Risks / Trade-offs

- **Stale asset marker** → Treat a missing or mismatched marker as a cache miss;
  bootstrap still verifies its own SHA-256 before execution.
- **Fingerprint implementation drift** → Include all regular files under the
  reviewed deployment directory and add shell tests for content and path
  changes.
- **Cache-hit misclassification** → The release command prints whether asset
  refresh was skipped, while the restricted helper remains responsible for
  image, migration, and health validation.

## Migration Plan

1. Add fingerprint calculation, cache-hit branching, and phase timing to the
   local release script.
2. Add optional fingerprint validation and persistence to bootstrap.
3. Extend shell regression tests and the deployment runbook.
4. Run strict OpenSpec validation, focused release tests, full release checks,
   and `git diff --check`.
5. Deploy once from protected `main`; the first deployment is a cache miss and
   establishes the marker, while later application-only releases use the fast
   path.

