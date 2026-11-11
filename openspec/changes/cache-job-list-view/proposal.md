# Proposal

## Why

Returning from a job detail page, trend dashboard, or another future statistics
entry currently destroys and recreates the job list view. That repeats the list
and filter-options requests and discards the user's already loaded results and
filter controls.

## What Changes

- Preserve the mounted job list view while navigating to other released views.
- Restore the existing list response, filter controls, and route-backed state
  when the user returns to the list, without changing router scroll behavior.
- Keep the current request behavior when the list is first opened or its route
  query changes.
- Do not cache job detail or statistics views, and do not change Serving APIs.

## Capabilities

### New Capabilities

- `job-list-view-cache`: Preserve the job list view across in-app navigation.

### Modified Capabilities

None.

## Impact

The application shell's route rendering and its navigation-shell tests change.
No backend, API, data, dependency, or URL-state contract changes.
