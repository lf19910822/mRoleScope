# Historical JD Comparison

## ADDED Requirements

### Requirement: Display-safe comparison

The historical detail response SHALL compare the node-day JD with the
request-time latest JD in Serving and SHALL expose only display-safe labels,
formatted values, and body fragments.

#### Scenario: Changed historical JD

- **WHEN** a historical detail belongs to a job marked `有改动` and a latest
  JD is available
- **THEN** the response includes dimension and body comparison data
- **AND** no SCD2 IDs, run IDs, parser versions, or database fields appear

#### Scenario: Confirmed closed historical JD

- **WHEN** a historical detail belongs to a `确认下线` job
- **THEN** the response contains the node-day JD without a comparison field

### Requirement: Deterministic body diff

The comparison SHALL return ordered `unchanged`, `deleted`, and `inserted`
fragments for each changed body section, preserving source text and list
positions.

#### Scenario: Replacement

- **WHEN** text is replaced inside one responsibility item
- **THEN** the old fragment precedes the new fragment in the same item

#### Scenario: Section addition or removal

- **WHEN** one body section is empty in one version
- **THEN** the complete added or removed section is represented as a change

### Requirement: Safe degradation

Historical content SHALL remain readable when latest lookup or comparison
calculation fails.

#### Scenario: Comparison unavailable

- **WHEN** historical content loads but comparison fails
- **THEN** the response remains successful with an unavailable comparison status
- **AND** the UI presents a non-blocking retry action
