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

Remaining before deployment: a desktop/390px acceptance of this final build
through the private API path, then a private, deployed detail endpoint
acceptance read. Deployment itself is not performed by this validation note.
