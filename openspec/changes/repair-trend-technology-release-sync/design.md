# Design

The fixed v0.4 successor release already validates every v0.4 result and
atomically advances `technology_analysis_publication_state`. Before committing
that transaction, it will also create or reuse the immutable trend formal
release whose technology component references the candidate v0.4 configuration.

The trend formal release uses the existing five-component identity and
`countUncoveredTrendFormalInputs` gate. If coverage is incomplete, the whole
v0.4 release transaction rolls back, preserving v0.3 as the active technology
publication. If the v0.4 configuration is already active, repeated release
invocation runs the same alignment step and can repair a stale formal trend
component without rewriting any immutable result.

The existing worker retry path remains valid for operational recovery, but a
successful fixed v0.4 release no longer depends on a later background analysis
run to make technology trends readable.
