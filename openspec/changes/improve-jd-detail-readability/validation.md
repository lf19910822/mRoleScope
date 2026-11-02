# Validation notes

## Production-safe range audit — 2026-11-02

The query ran through `root@118.89.68.77` inside the private PostgreSQL
container. It was aggregate-only and read-only: it did not select JD text,
aliases, identifiers, connection strings, or credentials, and it made no
mutation.

| Measure | Count |
| --- | ---: |
| Same-current-body technology evidence ranges | 876 |
| In-bounds ranges | 876 |
| Duplicate `(source_field,start,end)` tuples | 135 |
| Valid rows starting inside a preceding interval | 789 |
| Ranges retained by the serving sanitizer's deterministic non-overlap selection | 199 |

The serving behavior is intentionally best effort: a duplicate or overlap is
omitted from visual emphasis only. The JD body, its structured technology
tags, and unrelated data remain available.

## Local verification status

- Passed `go test ./internal/jobsync ./internal/jdserving ./internal/httpserver`
  and `go test ./...`.
- Passed `python3 -m unittest collector.test_collector` (6 tests).
- Passed `npm run test:unit -- --run` from `web/` (16 files, 48 tests) and
  `npm run build` from `web/`.
- Passed scoped Prettier and ESLint checks for every changed/new front-end
  file. Pre-existing unrelated formatting findings were not rewritten.
- Passed `openspec validate improve-jd-detail-readability --strict` and
  `git diff --check`.

The newly written migrated-PostgreSQL scenario is included but was skipped
locally because `TEST_DATABASE_URL` is not configured; the scoped CVM audit
above validates the production data shape without mutating it.

The local checks did not include an interactive browser viewport review. That
visual acceptance remains a manual product-review activity; it is not
represented as a completed automated validation task.

## Immutable-image deployment and private API acceptance — 2026-11-02

GitHub Actions run
[`41454559426`](https://github.com/lf19910822/mRoleScope/actions/runs/41454559426)
completed successfully for main commit `c98ea22f`. It ran the build, test,
network-boundary, and web-edge-boundary checks; pushed the four runtime images
by digest; then invoked the CVM's root-owned allowlist deployment command.

Post-deployment, the CVM API and PostgreSQL Compose services both reported
healthy. A short-lived, read-only, capability-dropped Python container joined
only the existing private backend network and read one current API list entry
and its detail. It printed only this structural summary:

| Check | Result |
| --- | ---: |
| List entries read | 1 |
| Non-empty JD body sections | 3 |
| Technology highlight intervals | 2 |
| Public highlight fields | `section`, `start`, `end` only |

The acceptance command did not print or persist JD text, public IDs,
credentials, connection strings, aliases, provenance, or internal processing
metadata. The temporary container was removed automatically.
