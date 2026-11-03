# Design

## Immutable v0.3 policy

`CandidateV03Seed` is derived from the completed v0.2 policy but gets a new
version label, source document, and content digest.  Only `go.01` (`Go`) and
`go.02` (`Golang`) change from conditional to reviewed candidates and use
ASCII case-insensitive matching with `ascii_identifier` boundaries.  Both map
to the existing stable `go` technology identity and canonical display name
`Go`.

The explicit boundaries prevent matches inside longer ASCII identifiers such
as `go-zero`, `go_micro`, and `gopher`; matching remains deterministic and
does not infer an additional technology from a related tool.

## Controlled lifecycle

The worker and root-owned adapters expose a closed v0.3 vocabulary:

1. seed the immutable candidate;
2. approve the fixed reviewed rule set;
3. prepare a full historical configuration;
4. run the isolated candidate analysis;
5. publish the fixed v0.3 upgrade.

Every one-shot service has only the private `backend` network.  The caller
cannot select a dictionary label, rule set, configuration, database URL,
image, SQL, path, or JD text.

## Upgrade gate

The v0.3 release operation only accepts the fixed v0.3 candidate and only
when the active pointer still references a fully published v0.2 configuration.
It requeues historical bodies, requires zero pending/missing results, replays
all evidence coordinates in PostgreSQL, validates the complete frozen v0.3
allowlist, then atomically publishes v0.3 and moves the pointer.  v0.2 is
marked `superseded` at the configuration level while its immutable dictionary,
results, evidence, and release attestation remain retained.

If the release fails or is incomplete, the v0.2 pointer remains active.

## Verification

Synthetic tests cover aliases across case variants, identifier boundaries,
approval eligibility, full candidate reanalysis, and the v0.2-to-v0.3 atomic
pointer transition.  No test fixture contains real JD content.
