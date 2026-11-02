# Spec Delta

## Purpose

Provide an efficient category-first way to browse formal technology keywords
while keeping the existing global keyword-search path and applied JD filter
semantics intact.

## ADDED Requirements

### Requirement: Categorized technology filter vocabulary

The existing filter-options response SHALL provide every selectable formal
technology keyword with one controlled, display-safe category value and
label. Categories and terms SHALL have deterministic dictionary ordering and
MUST NOT reveal parser, rule, audit, source, or raw-JD information.

#### Scenario: Current technology options are read

- **WHEN** the browser requests its filter vocabulary
- **THEN** each returned technology keyword has a stable category suitable for
  category browsing
- **AND** no new endpoint is required

#### Scenario: A future dictionary kind has no public category mapping

- **WHEN** a formal technology keyword belongs to an unrecognized dictionary
  kind
- **THEN** the response assigns it the controlled `其他技术` category rather
  than exposing the raw internal kind or omitting the selectable term

### Requirement: Category-first technology keyword selection

The JD browser SHALL render the technology-keyword menu as a two-pane
selector: categories on the left and terms in the active category on the
right. Terms SHALL remain independently multi-selectable, and choosing a
term SHALL continue to use the existing technology filter semantics.

#### Scenario: A visitor browses a category

- **WHEN** a visitor selects a technology category
- **THEN** the right pane shows that category's available terms in dictionary
  order

#### Scenario: Multiple terms are selected

- **WHEN** a visitor selects terms from one or more categories and applies
  the JD filters
- **THEN** the list request retains the same repeated `technology` values
- **AND** a JD matching any selected technology satisfies that dimension

#### Scenario: Browsing changes after a selection

- **WHEN** a visitor changes category after selecting a term
- **THEN** selections from the earlier category remain selected even when not
  visible in the active term pane

### Requirement: Global technology keyword search

The technology-keyword selector SHALL retain case-insensitive contains search
over the complete available technology vocabulary. Search results MUST not be
limited by the active browse category and SHALL remain selectable in
dictionary order.

#### Scenario: Search crosses categories

- **WHEN** a visitor enters a keyword fragment that matches terms in several
  categories
- **THEN** the selector shows the matching terms from all of those categories
- **AND** does not require the visitor to clear or change the active category

#### Scenario: Search is cleared

- **WHEN** a visitor clears the search text
- **THEN** the selector returns to the current category's dictionary-ordered
  browse list without removing any selected terms
