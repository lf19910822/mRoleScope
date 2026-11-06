# Spec Delta

## Purpose

Define historical node drill-down from a trend curve without exposing source
internals or creating a second snapshot store.

## ADDED Requirements

### Requirement: A node request is bound to one historical curve fact

The drill-down API SHALL accept the complete validated curve condition, node
date, node `count`, `page`, `page_size`, and an optional title search term. It
SHALL select that date's final successful published snapshot and rebuild the
matching job set from the same curve condition. The supplied count SHALL be
used only for an unsearched consistency check, never as a query predicate.

#### Scenario: Click a numeric node

- **WHEN** a reader clicks a node with a numeric count
- **THEN** the API returns jobs from that node date's final successful snapshot
- **AND THEN** no current membership, current body, or client-supplied job-ID
  list is used to manufacture the result

#### Scenario: Click a missing or zero node

- **WHEN** the node count is `null`
- **THEN** the client does not open a drill-down request
- **WHEN** the node count is trusted `0`
- **THEN** the client opens the fixed-size empty state “该日没有匹配岗位”

### Requirement: Drill-down lists are paginated and title-searchable

The API SHALL use `page` and `page_size`, default to 20, and reject a page
size above 100. It SHALL return `matched_total` and a deterministic list
ordering. Title search SHALL reuse the existing case-insensitive substring
semantics, trim surrounding whitespace, reset to page 1 on change, and remain
limited to the historical node collection.

#### Scenario: Search within a node

- **WHEN** a reader searches `Unreal` in a node list
- **THEN** only titles in that node's historical collection are searched
- **AND THEN** `matched_total` is the search-result total, not the chart count

### Requirement: User-facing list fields stay minimal

The transport MAY carry job identity and historical version locators needed to
open details, but the formal page SHALL render only the job title, one of
`仍有效` / `有改动` / `确认下线`, and the permitted detail actions. It SHALL
not render job IDs, body/metadata version IDs, run IDs, parser/dictionary
versions, audit states, or internal error codes.

#### Scenario: List response contains internal locators

- **WHEN** a list item is rendered
- **THEN** the user sees only its title, status, and allowed latest/history
  action
- **AND THEN** internal locators remain implementation data only

### Requirement: Current-status comparison is deterministic

For a historical job, the service SHALL compare the historical source body and
metadata version identities with the latest inventory. A job still present with
matching identities SHALL be `仍有效`; a present job with either identity
changed SHALL be `有改动`; a job with `confirmed_closed` or absent from the
latest inventory SHALL be `确认下线`.

#### Scenario: Historical job is unchanged

- **WHEN** the job remains in the latest inventory and both source identities
  match
- **THEN** the list marks it `仍有效` and offers only `查看最新`

#### Scenario: Historical job changed or disappeared

- **WHEN** either source identity differs while the job remains present
- **THEN** the list marks it `有改动` and offers history plus latest detail
- **WHEN** the job is confirmed closed or absent from the latest inventory
- **THEN** the list marks it `确认下线` and offers only historical detail

### Requirement: Historical detail resolves from SCD2 without current fallback

The detail service SHALL reuse the current public JD detail template and accept
either `latest` or the selected node date. Historical mode SHALL resolve the
body, metadata, and structured fields from the node date's final successful
publication through SCD2. If the historical version cannot be restored, it
SHALL return a safe unavailable result and MUST NOT silently serve current JD.

#### Scenario: Open changed historical JD

- **WHEN** a reader selects `查看历史 JD` for a changed or confirmed-closed
  item
- **THEN** the detail uses the node-date title, body, metadata, and structured
  fields
- **AND THEN** it labels the view `历史 JD · YYYY-MM-DD` without exposing IDs

#### Scenario: Historical restoration fails

- **WHEN** the node collection is known but its historical detail cannot be
  reconstructed
- **THEN** the service returns a safe unavailable state
- **AND THEN** it does not substitute the latest detail or remove the list item

### Requirement: Drill-down container preserves context responsively

The dashboard SHALL open one node-anchored popover/dialog on desktop and a
bottom sheet on narrow screens. The container SHALL retain fixed skeleton
dimensions while loading, support retry, and preserve node date, search term,
page, and scroll position when a detail panel closes. Clicking the same node
again SHALL close it; clicking another node SHALL reuse the same container.

#### Scenario: Desktop and mobile opening

- **WHEN** a numeric node is clicked on desktop
- **THEN** a bounded popover points back to that node and stays within the
  viewport
- **WHEN** the same action occurs at 390px width
- **THEN** a bottom sheet provides collapsed/expanded drag states and locks
  background scrolling

#### Scenario: Detail closes

- **WHEN** a reader closes the historical or latest detail panel
- **THEN** the original drill-down page, search, and scroll context reappears
- **AND THEN** no previous node click is added to browser history

### Requirement: Consistency and failure states are honest

For an unsearched request, if `matched_total` differs from the chart node
count, the client SHALL retain the service result and show a retryable “数据已
更新，请重新打开节点” notice. It SHALL not pad or truncate items. Search
results SHALL show only their service total. List pages may use a ten-minute
session cache keyed by node date, curve condition, search, and pagination;
refresh and explicit reload SHALL bypass it.

#### Scenario: Node count changed

- **WHEN** an unsearched response has a different total than the chart point
- **THEN** the list remains usable with a lightweight consistency notice
- **AND THEN** retry/reopen can obtain a fresh historical result
