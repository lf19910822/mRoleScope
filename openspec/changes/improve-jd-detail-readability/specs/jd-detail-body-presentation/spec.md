# Spec Delta

## Purpose

Make the normalized responsibility, requirement, and supplemental text easy to
scan while preserving every source-provided character whenever list structure
cannot be established deterministically.

## ADDED Requirements

### Requirement: Conservative JD body list recognition
The browser SHALL render a body section as a list only when it contains a
repeated, unambiguous top-level Arabic or Chinese ordinal marker, or repeated
spaced bullet marker. It MUST not infer item boundaries from prose sentences,
commas, semicolons, or technology punctuation.

#### Scenario: Repeated Arabic ordinals are present
- **WHEN** a body section contains a valid repeated Arabic ordinal sequence
- **THEN** the browser renders the corresponding content as separate ordered
  requirements

#### Scenario: Repeated bullet markers are present
- **WHEN** a body section contains two or more unambiguous spaced bullet
  markers
- **THEN** the browser renders the corresponding content as separate bullet
  requirements

#### Scenario: Prose contains incidental punctuation
- **WHEN** a body section contains sentence punctuation, version numbers, or
  hyphenated technology names without a valid repeated list marker
- **THEN** the browser does not split the prose into purported requirements

### Requirement: Lossless ambiguous-body fallback
The browser SHALL show a section whose markers are absent, incomplete, or
ambiguous as its complete normalized text in the existing readable body
presentation. It MUST retain any introductory text that precedes a recognized
list and MUST not hide or discard source content.

#### Scenario: No list structure is recognized
- **WHEN** a body section has no valid repeated list marker
- **THEN** the visitor sees the complete section text as one readable block

#### Scenario: A lead-in precedes a list
- **WHEN** text precedes otherwise valid list markers
- **THEN** that lead-in remains visible before the listed requirements

### Requirement: Readable requirement-item presentation
The browser SHALL present recognized requirement items as semantic ordered or
unordered lists with sufficient visual separation to scan each item. The
presentation MUST remain legible at desktop and 390px viewport widths and
must not change the JD's wording.

#### Scenario: A visitor reads a long requirements section
- **WHEN** the section has multiple recognized entries
- **THEN** each entry has a distinct visual row and remains selectable as text

#### Scenario: A narrow screen displays list items
- **WHEN** the viewport is 390px wide
- **THEN** rows wrap without horizontal scrolling or clipped requirement text
