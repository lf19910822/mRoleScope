# Spec Delta

## ADDED Requirements

### Requirement: Go aliases are one case-insensitive programming-language identity

The system SHALL register `Go` and `Golang` as explicit aliases of the single
canonical `Go` programming-language identity in an immutable dictionary
version.  Both aliases SHALL use ASCII case-insensitive matching and ASCII
identifier boundaries; the system SHALL not infer Go from a longer identifier,
an unregistered spelling, or a related tool.

#### Scenario: Registered Go aliases vary only in ASCII case

- **WHEN** an enabled v0.3 rule encounters `Go`, `go`, `GOLANG`, or `golang`
  as an independent ASCII identifier in an eligible JD body field
- **THEN** it SHALL record the same canonical `Go` technology identity

#### Scenario: A Go-looking string is inside a larger identifier

- **WHEN** an eligible JD body field contains `go-zero`, `go_micro`, or
  `gopher`
- **THEN** the system SHALL not create a `Go` mention from that substring

### Requirement: Published dictionary corrections use an immutable upgrade

The system SHALL not alter an already published technology dictionary or its
frozen rules to add Go aliases.  It SHALL use a new candidate dictionary,
reanalyze all available historical body versions, validate its fixed approved
rule set and evidence coordinates, and atomically move the active pointer
only after the candidate passes the release gate.

#### Scenario: v0.3 is incomplete or invalid

- **WHEN** v0.3 has pending work, missing results, invalid evidence, or an
  incomplete approved rule set
- **THEN** the active v0.2 configuration SHALL remain active

#### Scenario: v0.3 passes the complete gate

- **WHEN** v0.3 has complete historical results and valid evidence for its
  fixed approved rule set
- **THEN** the system SHALL publish v0.3 and atomically advance the active
  configuration pointer
- **AND THEN** v0.2 results, evidence, dictionary, and release attestation
  SHALL remain preserved as superseded history
