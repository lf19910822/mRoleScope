# Design

## Context

See [proposal.md](proposal.md). Source observations, presence periods, content
versions, and formal JD-modeling results are already versioned and immutable.
The API currently exposes only health endpoints, while `web/` is a Vue starter
application. The remote API is bound to CVM loopback, and there is no web edge
service.

The product decisions recorded during exploration are:

- The list shows the latest data; future charts must nevertheless be able to
  query versioned historical facts.
- A JD has one main job family and zero or more other matched job families;
  they are not a parent-child taxonomy.
- Structured dimensions, not JD-body full-text, drive search. Values within
  one dimension use OR; different dimensions use AND. Title supports
  case-insensitive substring and typo-tolerant fuzzy matching.
- Education uses 专科及以下、本科、硕士、博士 and preserves lower/upper-bound
  direction. The public interface calls this `学历要求`.
- The initial list defaults to effective jobs and can select effective,
  confirmed-closed, or all. A transient source absence is never a closure.
- The serving layer returns a prior completed formal value for the same job
  while a current version is still being processed. This implementation detail
  is not shown in the browser or charts.
- The public release requires a domain, HTTPS, and authenticated access; a raw
  IP is not an entrypoint.
- The supplied Ocean Breeze Naive UI theme is the visual baseline: its light
  palette, sky-blue/cyan actions, filled controls, generous rounded corners,
  and readable data-table spacing guide the first browser release. Dark-mode
  tokens are retained for a later explicit theme-mode decision.

## Goals / Non-Goals

**Goals:**

- Build a DWD/serving query boundary over the latest published job inventory
  without altering ODS collection semantics.
- Retain the upstream run, presence-period, metadata/body-version, and formal
  result identities necessary for later date/version views.
- Give the Vue client a typed, paginated list/detail REST contract and a
  same-origin secure deployment route.
- Make filters and sorts queryable with predictable database cost as the
  inventory grows.

**Non-Goals:**

- No chart pages, historical-difference UI, AI semantic projection, or body
  full-text search.
- No new source crawler fields, changes to source-publishing transactions, or
  failure of source synchronization because serving reconciliation fails.
- No public raw-IP endpoint, anonymous visitor access, registrar automation,
  payment handling, or identity verification handling.

## Decisions

### 1. Resolve current data from versioned facts at the DWD boundary

The API will query a DWD serving read model whose membership is anchored to the
latest `observation_scope_publication_state` run and whose activity state comes
from `job_presence_periods`. It will retain references to the observed metadata
and body version plus the formal source result selected for each dimension.

The resolver first uses a formal result for the observed version; if absent, it
selects the latest completed result for the same logical job and dimension. It
never reads AI semantic results. A missing value remains unavailable rather
than blocking the row. Serving reconciliation is best-effort and retryable,
like other downstream processing, so source publication remains independent.

An append-only per-published-run membership/read-model history will be kept
alongside the current lookup. Immutable membership rows retain only the source
and lifecycle identities. A separate append-only per-dimension resolution
record points at each selected formal result; a later completed result appends
a newer selection instead of mutating a historical membership. The current
resolver prefers a selection for the observed version, then a fallback
selection, and finally reports the field unavailable. This avoids treating a
rewritten current view as the only source for later charts and version
comparison.

Alternative considered: make the browser join all normalized source and
analysis tables directly. It avoids a new projection but duplicates complex
fallback logic across endpoints and leaves no bounded query contract for
future charts.

### 2. Define list filters against formal, display-safe dimensions

The list supports title, main/other job-family match, deterministic technology
identity, education bounds, minimum experience, deterministic title-project
assignment, location, activity state, first-discovered date, and source urgent
flag. `项目` means the formal title-project assignment, not source
`projectName`, whose current value is the unhelpful recruitment label
`社会招聘`.

Within a filter dimension selected values are ORed; across dimensions they are
ANDed. List pagination has a stable deterministic tiebreaker. Experience and
education sort use explicit rank columns and put unavailable values after
configured values in both directions. Title search is title-only and combines
case-insensitive substring matching with PostgreSQL fuzzy matching; exact and
substring matches rank before fuzzy-only matches.

Alternative considered: accept one global text query. It conflicts with the
decision not to search responsibilities, requirements, or supplemental text,
and obscures which modeled dimension caused a match.

### 3. Keep public API DTOs separate from provenance and audit facts

The HTTP layer returns normalized display DTOs, opaque JD identifiers, filter
options, pagination metadata, and safe client errors. It does not expose
version IDs, parser/dictionary identities, source raw payloads, fallback state,
analysis coverage, audit events, AI output, or credentials. Detail reads the
current normalized responsibility, requirement, and optional supplemental
sections only.

Alternative considered: return repository rows as JSON. That would couple the
browser to evolving tables and leak processing and provenance semantics that
are intentionally internal.

### 4. Use a dedicated static edge service with initial credential protection

Add a dedicated web-edge image that serves the built Vue application and
proxies `/api/` to the Go API over the existing internal Docker network. It
terminates TLS for the approved domain and applies initial HTTP Basic
Authentication before both static and API delivery. The password is a strong,
unique secret stored only as a bcrypt hash in root-owned CVM runtime
configuration; it is never in Git, Docker image layers, browser source, or
GitHub Actions logs.

The Go API remains loopback/internal; the edge is the sole intended public
listener. Same-origin delivery eliminates the need for a public CORS policy.
The edge supports a later swap to an identity-provider access layer without
changing `/api/` or the frontend origin contract.

Alternative considered: expose the API directly with CORS and let the Vue app
be hosted elsewhere. This duplicates security controls, creates origin drift,
and makes credentials/API policy harder to contain. An SSH-only tunnel was
also rejected because the product now requires a domain-backed secure release.

### 5. Gate public exposure on manual domain and jurisdiction prerequisites

Before deployment enables public 80/443, the operator must acquire the chosen
domain, complete the registrar's ownership requirements, enable account MFA,
turn on auto-renew and transfer lock, and delegate DNS. Since the CVM is in
mainland China, the operator must also determine and complete any provider or
regulatory filing required before public web hosting (for example, ICP
procedures when applicable). No repository code automates these actions or
stores their credentials.

The deployment configuration receives only the approved hostname and the
bcrypt password hash through root-owned runtime files. DNS points only the
hostname to the edge after TLS/access-control preflight succeeds; raw-IP
bookmarks are not documented as product access paths.

## Risks / Trade-offs

- [A source publish or analysis result arrives before reconciliation] → retain
  the prior completed formal value when possible; run reconciliation
  independently and never fail source publication.
- [A filter query regresses as history grows] → query current DWD membership,
  use `EXPLAIN (ANALYZE, BUFFERS)` against representative data, add targeted
  indexes, and cap validated page sizes.
- [Fuzzy title matching produces a surprising result] → rank exact/substrings
  first, use a fixed tested threshold, and do not expand matches into unseen
  synonyms or JD-body text.
- [HTTP Basic credentials are leaked or become insufficient] → use a unique
  long secret represented only by a bcrypt hash, rate-limit at the edge, and
  retain a replaceable edge boundary for a later identity-provider migration.
- [A domain is purchased but public hosting prerequisites are incomplete] →
  keep the API loopback-only and do not create public listeners or DNS records
  until the manual gate is confirmed.
- [TLS issuance or edge startup fails] → preserve the existing API deployment
  and roll back the edge independently; no DNS switch occurs until local and
  external HTTPS checks pass.

## Migration Plan

1. Complete the manual domain prerequisite task, including account and
   registrar hardening and hosting-jurisdiction confirmation. Record only the
   public hostname and completion status in the operational runbook.
2. Add and test the serving schema/reconciliation and API locally; backfill it
   from durable published facts without changing ODS rows.
3. Build and test the Vue list/detail application against the API contract.
4. Build the web-edge image and add it to the immutable-image deployment
   allowlist. Configure hostname and access-password hash in root-owned CVM
   runtime configuration.
5. After the manual gate is complete, deploy API, migration, worker, and web
   edge; verify authenticated HTTPS list/detail access and that API/PostgreSQL
   stay non-public. Only then create the production DNS record.
6. Roll back by removing the DNS record or stopping the edge service; API,
   source sync, immutable history, and analysis jobs continue independently.
