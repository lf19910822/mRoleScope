# Design

## Decision

Render the router's current component through a scoped `KeepAlive` in the
application shell and include only `JobListView`. The list component is
deactivated, not unmounted, when the route changes to detail, trends, or a
future statistics route; returning activates the same instance.

## Boundaries

- `JobListView` remains the owner of list response, filter options, draft
  controls, and route-query synchronization.
- `JobDetailView` and statistics views remain ordinary route components and are
  not retained by this change.
- The route watcher ignores non-list routes, reuses the last requested query on
  return, and still requests a different query. Failed requests may be retried
  on a later visit. The sidebar remembers the last list URL during this app
  session; explicit links to a different list query remain authoritative.
- No response cache, TTL, invalidation policy, or API change is introduced;
  component retention is the cache boundary for this requirement. A browser
  reload creates a new instance and requests fresh data. Without a reload or
  changed query, retained results can remain stale for the page session.

## Navigation Semantics

The browser's saved history position continues to be handled by the existing
router scroll behavior. `KeepAlive` preserves the list DOM and reactive state;
the router remains the authority for the list URL and query state.

## Verification

Add an application-shell integration test that mounts the real router view,
enters the list, navigates to detail and back, and asserts the list and
filter-options endpoints are each called once. The test also changes a list
query and asserts that a new list request still occurs.
