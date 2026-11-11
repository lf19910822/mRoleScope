# Spec Delta

## Purpose

Make the normalized responsibility, requirement, and supplemental text easy to
scan while preserving every source-provided character whenever list structure
cannot be established deterministically.

## ADDED Requirements

### Requirement: Conservative JD body list recognition

The browser SHALL render a body section as a list when it contains one or more
unambiguous top-level Arabic or Chinese ordinal markers, or repeated spaced
bullet markers. It MUST not infer item boundaries from prose sentences,
commas, semicolons, or technology punctuation.

#### Scenario: Arabic ordinals are present

- **WHEN** a body section contains one or more valid Arabic ordinal markers
- **THEN** the browser renders the corresponding content as separate ordered
  requirements

#### Scenario: Explicit source headings separate restarted ordinal groups

- **WHEN** an otherwise consecutive Arabic ordinal sequence restarts at `1`
  after a recognized source heading of `基础` or `加分项` (with an optional
  colon)
- **THEN** the browser renders the source heading before a separate ordered
  requirement group
- **AND** the preceding requirement ends before that heading

#### Scenario: An explicit source group skips an ordinal

- **WHEN** a recognized `基础` or `加分项` source group starts at `1` and a later
  ordinal increases while skipping a value, such as `1. → 3.`
- **THEN** the browser renders the source heading and each proven marker as a
  separate ordered requirement
- **AND** it retains the displayed source markers without inventing the
  missing ordinal
- **AND** the same gap outside a recognized source group remains fallback text

#### Scenario: An ordinal reset has no explicit source heading

- **WHEN** an Arabic ordinal sequence restarts at `1` without a recognized
  source heading
- **THEN** the browser preserves the complete section as fallback text rather
  than inventing a group boundary

#### Scenario: A single ordinal marker is present

- **WHEN** a body section has only one unambiguous top-level ordinal marker,
  such as `1、具有大规模集群运维经验`
- **THEN** the browser renders its source text as one ordered requirement

#### Scenario: A parenthesized ordinal introduces a numeric requirement

- **WHEN** a repeated Arabic ordinal uses a closing parenthesis and the
  following requirement begins with a number, such as `2）2年以上相关经验`
- **THEN** the closing-parenthesis ordinal remains a valid list marker
- **AND** an enumeration-comma ordinal with numeric content, such as
  `1、3年以上游戏客户端开发经验`, remains a valid list marker
- **AND** isolated dot-style decimal or version fragments whose dot is followed
  by a digit remain ineligible as ordinal markers

#### Scenario: A numeric-dot ordinal has one source whitespace before its dot

- **WHEN** a valid top-level Arabic ordinal uses one horizontal whitespace
  character before a dot or full-width dot, such as `2 .` or `2 ．`
- **AND** the dot is not followed by a digit
- **THEN** the browser renders it as an ordered requirement while retaining its
  source text and visible numeral
- **AND** a decimal/version fragment such as `1 .28` remains ineligible

#### Scenario: The first proven ordinal pair establishes punctuation style

- **WHEN** the first two unambiguous consecutive Arabic or Chinese ordinals
  use the same normalized punctuation style, such as `1.` and `2.`
- **THEN** later ordinal candidates in the same section are eligible only when
  they use that style, treating ASCII/full-width dot variants and optional
  source whitespace as the same dot style
- **AND** a candidate with another punctuation style, such as `5）` inside a
  dot-style sequence, remains source text rather than becoming a list item
- **AND** a later valid marker remains usable when the ignored candidate fills
  the otherwise missing serial, while an unbridgeable skip still falls back
  to the complete source text

#### Scenario: Consecutive source markers contain numeric-leading items

- **WHEN** a numeric-dot candidate whose dot is immediately followed by a
  digit occurs inside a source-proven consecutive ordinal sequence and is
  bounded by unambiguous ordinal markers on both sides
- **THEN** the browser renders it as its own ordered requirement without
  classifying or whitelisting the following prose
- **AND** an isolated candidate, or one at either edge of such a sequence,
  remains fallback text so decimal and version fragments are not presented as
  requirements

#### Scenario: Repeated bullet markers are present

- **WHEN** a body section contains two or more unambiguous spaced bullet
  markers
- **THEN** the browser renders the corresponding content as separate bullet
  requirements

#### Scenario: An ordinal responsibility contains compact hyphen subentries

- **WHEN** a proven top-level ordinal item contains two or more ASCII hyphens
  at valid list boundaries and each hyphen is immediately followed by
  non-whitespace source text
- **THEN** the browser retains the ordinal item's preamble as its parent
  responsibility and renders the hyphen entries as a semantic nested bullet
  list
- **AND** a single compact hyphen or a hyphenated technology term remains
  ordinary source text

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

- **WHEN** a body section has no valid list marker
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

### Requirement: Persistent detail-list navigation

The browser SHALL keep the `返回岗位列表` action available while a visitor
reads a long job detail page. The control MUST preserve the existing browser
history return behavior, remain above the detail content without obscuring its
title or body, and stay reachable on narrow screens.

#### Scenario: A visitor scrolls through a long detail

- **WHEN** the visitor scrolls away from the top of a job detail page
- **THEN** the `返回岗位列表` action remains visible in the sticky reading
  rail beside the body
- **AND** activating it returns to the previous job-list route

#### Scenario: A narrow screen displays the fixed return action

- **WHEN** the detail page is displayed at a narrow viewport
- **THEN** the sticky action avoids the mobile navigation control and the detail
  title remains visible below it

### Requirement: Static source-item reading

A recognized source-list row SHALL remain a complete, static reading surface
for every pointer type. It MUST NOT scale, rotate, translate, or add a
pointer-tracking light effect, and it MUST NOT attach pointer-motion handlers
solely for visual response. Its text remains selectable and its list semantics
remain unchanged.

#### Scenario: A visitor moves across a source-list row

- **WHEN** a mouse or touch pointer moves across a recognized list row
- **THEN** the complete source item remains visually static and readable
