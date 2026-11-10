# Job Last Modified

## ADDED Requirements

### Requirement: Last modification is a derived observed date

The list SHALL provide the Shanghai business date of the latest qualifying
daily history event, using the existing history timeline's source-version and
daily selection semantics. An ordinary modification and a reopening qualify.

#### Scenario: Reopening without source change

- **WHEN** a confirmed-closed job reopens with unchanged source versions
- **THEN** its reopening business date qualifies as its latest modification date

#### Scenario: Ordinary history modification

- **WHEN** an existing history timeline modified event occurs after the last
  qualifying event
- **THEN** that event's business date becomes the latest modification date

#### Scenario: Non-qualifying events

- **WHEN** an observation only discovers a job, confirms closure, changes a
  parser or dictionary, or repeats an unchanged source version
- **THEN** it does not produce a modification date

#### Scenario: No known modification

- **WHEN** retained history contains no modification or reopening
- **THEN** the API returns a null date and the card displays `-`

#### Scenario: Gap and repaired dates

- **WHEN** a daily observation is missing or a historical observation is repaired
- **THEN** selection and comparison follow the existing history timeline rules
- **AND** no unobserved event is invented

### Requirement: Last-modified ordering and display

The job list SHALL display the last modification date in the existing card
metadata and support ascending or descending order before pagination.

#### Scenario: Newest or oldest first

- **WHEN** a reader sorts by the last modification date
- **THEN** known dates are ordered in the selected direction
- **AND** jobs with no known date appear after jobs with known dates

#### Scenario: Ordered multi-rule sort

- **WHEN** the reader combines the new sort with existing sort rules
- **THEN** earlier rules retain higher priority, up to four distinct rules
- **AND** title and public identifier break any remaining ties across pages

#### Scenario: Safe display

- **WHEN** a card displays its last modification date
- **THEN** it exposes no internal version, processing, audit, or repair details
- **AND** the absence of a date is not replaced by the first-discovered date
