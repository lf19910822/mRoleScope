# Spec Delta

## Purpose

Give the JD browser a responsive, durable navigation shell so released
functions have a consistent home without advertising unfinished pages as if
they were usable product features.

## ADDED Requirements

### Requirement: Released-function navigation

The browser SHALL render a persistent primary navigation area on the left at
desktop widths. It MUST show `岗位筛选` as the only navigable function until
another destination is released, and it MUST link that entry to the JD list.

#### Scenario: Desktop visitor opens the browser

- **WHEN** the viewport has desktop width
- **THEN** the visitor sees a left navigation area containing an active
  `岗位筛选` entry that opens the JD list

#### Scenario: No future feature is released

- **WHEN** charts or another destination have not been implemented
- **THEN** the navigation does not show a clickable, disabled, or placeholder
  entry for that feature

### Requirement: Navigation state across JD reading

The navigation SHALL mark `岗位筛选` as the active function for both the JD
list and a JD detail reached from that list.

#### Scenario: Visitor reads a JD detail

- **WHEN** a visitor opens a detail route from the JD list
- **THEN** the navigation continues to identify `岗位筛选` as the active
  function

### Requirement: Responsive navigation access

The browser MUST make the released-function navigation available without a
permanent left column on narrow viewports. Its control MUST have an accessible
name and must not cover the page content when closed.

#### Scenario: Narrow viewport visitor opens navigation

- **WHEN** a visitor uses a narrow viewport and activates the navigation
  control
- **THEN** the `岗位筛选` destination is available and the visitor can dismiss
  the navigation to return to unobscured content

#### Scenario: Narrow viewport visitor follows the destination

- **WHEN** a visitor chooses `岗位筛选` from the narrow navigation
- **THEN** the browser navigates to the JD list and closes the navigation
