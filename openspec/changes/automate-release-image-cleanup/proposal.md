# Proposal

## Why

The CVM builds nothing locally, but each immutable deployment pulls four new
images. Its current runtime needs only a small subset of those layers, while
historical release images accumulate over time. The deployment process needs a
bounded, release-aware cleanup step that controls disk growth without risking
the PostgreSQL volume, another Docker workload, the running release, or a
known-good rollback.

## What Changes

- After a verified deployment is recorded, retain the four images of the new
  release and the four images recorded by the immediately preceding successful
  release.
- Remove only unreferenced, immutable `mrolescope-api`, `mrolescope-migrate`,
  `mrolescope-worker`, and `mrolescope-web-edge` images from the configured
  Docker Hub namespace.
- Treat cleanup as best effort after a successful deployment: a cleanup failure
  is logged but cannot relabel a healthy, recorded release as failed.

## Impact

- The root-owned CVM deployment helper and its isolated shell tests.
- No application API, database schema, data volume, public port, or image
  build contract changes.
