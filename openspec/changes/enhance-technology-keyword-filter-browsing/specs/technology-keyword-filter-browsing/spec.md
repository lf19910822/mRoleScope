# Spec Delta

## Purpose

Make the formal technology vocabulary practical to browse without changing
the established, structured JD filtering contract or exposing pipeline
implementation details.

## ADDED Requirements

### Requirement: Categorized technology filter options

The existing JD filter-options response SHALL include a controlled public
category for every selectable technology keyword. It MUST map unknown future
dictionary kinds to a controlled other category and MUST NOT expose raw
dictionary kind values, parser metadata, source text, or audit data.

#### Scenario: Known technology kind

- **WHEN** the current technology vocabulary includes a keyword whose kind is
  mapped by the Serving boundary
- **THEN** its filter-option entry includes that controlled category's value
  and display label

#### Scenario: Unknown future kind

- **WHEN** the current vocabulary includes a keyword with an unrecognized
  technology kind
- **THEN** its filter-option entry remains selectable in the `其他技术`
  category without exposing the raw kind

### Requirement: Category-first keyword browsing

The JD browser SHALL let a visitor browse technology keywords by category,
select multiple terms, and search keyword labels globally without losing prior
selections. It MUST continue to use the existing repeated `technology` query
parameters when filters are applied.

#### Scenario: Browse a category

- **WHEN** a visitor selects a technology category
- **THEN** the selector displays that category's currently available terms in
  the order returned by the formal vocabulary

#### Scenario: Search across categories

- **WHEN** a visitor enters a non-empty keyword search string while any
  category is active
- **THEN** the selector shows case-insensitive matching terms from all
  categories and retains selections made before the search

#### Scenario: Apply selected terms

- **WHEN** a visitor applies multiple selected technology terms with other
  filters
- **THEN** the browser preserves the established same-dimension union and
  cross-dimension intersection semantics
