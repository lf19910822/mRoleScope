# Spec Delta

## Purpose

Preserve the loaded job list experience while users move between the list,
detail, trend, and future statistics views.

## ADDED Requirements

### Requirement: Preserve the job list view across navigation

The application SHALL preserve the mounted job list view when the user
navigates from the job list to a job detail page, the trend dashboard, or
another future statistics view.

#### Scenario: Return from job detail

- **WHEN** a user opens a job detail page from the job list and returns through
  in-app navigation
- **THEN** the existing job list response remains visible without another job
  list request
- **AND** the existing filter options remain available without another
  filter-options request
- **AND** the user's list route state and scroll position are preserved

#### Scenario: Return from a statistics view

- **WHEN** a user returns through the sidebar from a statistics view to a
  previously visited job list
- **THEN** the sidebar restores the last list URL and its mounted view without
  repeating that list's requests
- **AND** explicit links specifying another list query retain their meaning

### Requirement: Preserve request semantics for list state changes

The application SHALL continue to request a new job list when the job list
route query changes to represent a different filter, sort, page, or page size.

#### Scenario: Apply a different list query

- **WHEN** the user changes filters, sorting, pagination, or page size
- **THEN** the existing route watcher requests the corresponding list state
- **AND** the preserved view does not serve results for a different route query
