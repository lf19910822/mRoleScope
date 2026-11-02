# Proposal

## Why

The repository has versioned source and JD-modeling facts, but the only HTTP
endpoints are health checks and the Vue application remains its starter screen.
Users therefore cannot search or inspect the latest JD inventory, despite the
data pipeline already producing the relevant formal fields.

This change introduces a fast, read-only serving path for the latest published
JD data. It makes the formal data usable now while retaining the historical
links needed by later date-based charts and version comparisons.

## What Changes

- Add a version-aware DWD/serving read model and REST JSON API for the latest
  published JD list, structured filter options, pagination, sorting, and one
  JD detail view.
- Add the Vue job-list and JD-detail experience, including composable filters
  for title, job-family hits, formal technology keywords, education,
  experience, project labels, location, active state, first-discovered date,
  and urgent status.
- Use the latest formal result for each current source version when available;
  otherwise serve the same job's most recent completed formal result for that
  dimension. This is transparent to viewers and remains auditable internally.
- Add DWD/serving indexes or read structures necessary for responsive filtered
  lists without changing the ODS collection contract.
- Acquire an approved domain before release, then package the static frontend
  behind a same-origin HTTPS reverse proxy with access control. The raw CVM IP
  is never the product entrypoint.
- Treat domain registration, account/registrar hardening, DNS delegation, and
  any applicable mainland-hosting filing as explicit manual deployment
  prerequisites; do not collect registrar, payment, or identity credentials.
- Exclude charts, historical version comparison, and full-text JD-body search
  from this change.

## Capabilities

### New Capabilities

- `jd-serving-search`: provides a stable, latest-data JD search and detail
  contract over versioned source and formal-modeling facts.
- `jd-browser-ui`: provides the read-only Vue list and detail interface that
  consumes the serving contract.
- `secured-domain-access`: provides the domain-bound, HTTPS-protected,
  same-origin deployment path for the JD browser.

### Modified Capabilities

- None.

## Impact

- Go HTTP routing, a dedicated serving-query/repository layer, migrations for
  DWD/serving performance where needed, and API tests.
- The Vue/Vite application, its tests, and its production build packaging.
- Remote-development Compose and verified deployment assets, while preserving
  the existing ODS synchronizer, API loopback boundary, and PostgreSQL-only
  deterministic analysis pipeline.
