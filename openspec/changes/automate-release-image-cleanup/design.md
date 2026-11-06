# Design

## Retention boundary

Before a deployment overwrites `current.env`, the helper reads its complete,
validated four-image release record as the rollback generation. Once the new
API (and, when enabled, Web Edge) passes health checks, it writes the new
`current.env` and success record. Cleanup then retains both complete image
sets: the just-verified release and that captured predecessor.

The predecessor is read from the root-owned `current.env`, rather than inferred
from filename ordering, so simultaneous records cannot alter the retention
choice. If an existing release record is malformed, has an unexpected owner or
mode, or lacks any of the four allowed digests, cleanup fails closed for that
deployment while the already-verified service remains available.

## Safe candidate selection

The helper lists local Docker repositories and image IDs, then resolves each
managed image ID through Docker's `RepoDigests`. This is required because an
image pulled by digest may display `Tag=<none>` and `Digest=<none>` in
`docker image ls` while still retaining its authoritative `RepoDigests` entry.
It considers only digest-addressed images whose repository is exactly one of
the four `mrolescope-*` repositories in the configured namespace. It never
invokes a global `docker system prune`, never uses `--force`, and never selects
PostgreSQL, unnamed images, foreign repositories, containers, volumes,
networks, build cache, or runtime secrets.

For each non-retained candidate, the helper compares its image ID with every
container (including stopped containers). Any referenced image is skipped. A
failed individual removal is logged and processing continues.

## Lifecycle

1. Validate four immutable input digests and capture the prior complete
   `current.env` release, if one exists.
2. Pull, migrate, start, and health-check exactly as today.
3. Record the verified new release and retain release-record audit history.
4. Best-effort remove only safe, unreferenced candidates outside the current
   and immediately preceding release.

The image cleanup comes after release recording and sets deployment success
before it starts. Therefore cleanup cannot trigger an API rollback or convert a
successful database migration into a failed release record.

## Non-goals

- No deletion of PostgreSQL volumes or other Docker data.
- No zero-image runtime: running containers retain their required local layers.
- No cleanup before health checks or for a failed deployment.
- No attempt to keep arbitrary old releases available during a Docker Hub
  outage; the explicit retention window is current plus one predecessor.
