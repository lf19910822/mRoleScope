# Spec Delta

## Purpose

Provides reproducible JD project and direction tags from approved title
keywords, preserving provenance without inferring project ownership from AI,
the JD body, the technical stack, or similarly named source metadata fields.

## ADDED Requirements

### Requirement: Project dictionary entries are human-approved and versioned
The system SHALL derive project tags only from an immutable, human-approved
dictionary version. Every entry SHALL have a stable identifier, display name,
classification (`named_game` or `source_direction`), enabled state, and one
or more explicitly registered keywords. It SHALL not create or alter entries
from AI output, source metadata, title text, or technical mentions.

#### Scenario: An approved named game is present in a title
- **WHEN** an enabled named-game keyword exactly matches a normalized title
- **THEN** the system records an assignment to that stable dictionary entry
- **AND THEN** it classifies the assignment as `named_game`

#### Scenario: A source direction label is present in a title
- **WHEN** an enabled source-direction keyword exactly matches a normalized title
- **THEN** the system records an assignment to that stable dictionary entry
- **AND THEN** it does not represent the label as a confirmed game

#### Scenario: An unregistered title suffix is observed
- **WHEN** a title contains text that is not an enabled dictionary keyword
- **THEN** the system produces no formal assignment from that text
- **AND THEN** it does not create a dictionary entry or invoke an AI provider

### Requirement: Title matching is deterministic and evidence-backed
The system SHALL match only normalized title text using each dictionary
keyword's explicit case and boundary policy. It SHALL preserve every distinct
simultaneous entry match and record the matched keyword plus its zero-based
Unicode-code-point half-open range. It SHALL not match JD body fields.

#### Scenario: An English keyword differs only in case
- **WHEN** a title contains an enabled case-insensitive English keyword using different letter case
- **THEN** it matches the same dictionary entry

#### Scenario: An English keyword occurs inside a longer identifier
- **WHEN** a title contains an enabled English keyword as part of a longer ASCII identifier
- **THEN** the system does not assign the entry unless its registered boundary policy permits that match

#### Scenario: Two entries match one title
- **WHEN** a title contains keywords for two distinct enabled entries
- **THEN** the result retains two assignments with independent evidence
- **AND THEN** it does not choose an entry by scan order or infer an equivalence between them

### Requirement: Completed results distinguish unrecognized titles from pending work
The system SHALL persist one completed deterministic result for each normalized
title identity and immutable parser/dictionary identity, including when no
enabled keyword matches. An empty completed assignment set SHALL mean
`unidentified`, not a default project, a source project field value, or an
unprocessed title.

#### Scenario: A title has no matching keyword
- **WHEN** an eligible normalized title matches no enabled dictionary keyword
- **THEN** the system persists a completed result with no assignments
- **AND THEN** downstream reads report the title as `unidentified`

#### Scenario: An unchanged unrecognized title is seen again
- **WHEN** the same title identity is observed in a later successful source run under the same parser and dictionary identities
- **THEN** the system reuses its completed empty result
- **AND THEN** it does not create duplicate work or call a provider

### Requirement: Incremental processing follows title and dictionary identity
The system SHALL reprocess only title identities that lack a completed result
for the active parser and dictionary identities. A source metadata change that
does not alter the normalized title SHALL reuse the prior result; a title or
dictionary identity change SHALL create a distinct historical result.

#### Scenario: Only location metadata changes
- **WHEN** a later source observation changes location while retaining the same normalized title
- **THEN** the system reuses the existing project-tag result
- **AND THEN** it does not create a new project-tag result solely for that metadata change

#### Scenario: A job title changes
- **WHEN** a later source observation has a different normalized title for the same stable job
- **THEN** the system creates or reuses a distinct project-tag result for the new title identity
- **AND THEN** it retains the earlier title result unchanged

#### Scenario: The project dictionary changes
- **WHEN** a new immutable dictionary version is activated
- **THEN** the system processes available historical title identities for that version
- **AND THEN** it retains results from earlier dictionary versions for audit

### Requirement: Formal project reads exclude source fields and body mentions
The formal JD project read SHALL return assignments from the completed
deterministic title result and its dictionary classification. It SHALL not
substitute `projectId`/`projectName`, a JD-body mention, a technology result,
or an AI projection when the title is unidentified or has no completed result.

#### Scenario: A source project field contains a recruitment label
- **WHEN** source metadata has a project name but the title has no dictionary match
- **THEN** the formal project read remains `unidentified`

#### Scenario: A JD body mentions a game but the title does not
- **WHEN** a body field mentions a dictionary game name while the title has no dictionary match
- **THEN** the formal project read remains `unidentified`
- **AND THEN** it does not create a title assignment from the body mention

### Requirement: Processing is independent from AI and body analysis
The standard fixed analysis operation SHALL process deterministic project tags
after a successful source synchronization without model credentials or provider
calls. A failure or absence of technology or AI processing SHALL not prevent
project-tag processing, and project-tag processing SHALL not block source
synchronization.

#### Scenario: Technology analysis is unavailable
- **WHEN** the source synchronization succeeds but technology analysis is unavailable
- **THEN** deterministic project-tag processing still attempts eligible title identities
- **AND THEN** source synchronization remains successfully published
