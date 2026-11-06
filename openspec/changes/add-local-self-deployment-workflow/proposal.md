# Proposal

## Why

The existing release boundary on the CVM is safe, but its build-and-deploy
entry point depends on GitHub Actions. The project now needs a maintainable
operator-owned path: the development machine runs the same release checks,
builds the four immutable images locally, pushes them to Docker Hub, and asks
the CVM to deploy only their digests.

## What Changes

- Add a local release verifier and an explicit `--deploy` release command.
- Require a clean, up-to-date `main` checkout and an identical revision label
  on API, migration, worker and Web Edge images.
- Before each release, refresh the CVM's reviewed root-owned deployment assets
  from that immutable `main` commit, then invoke its existing restricted
  four-digest deployment helper.
- Document the operator procedure and make it discoverable to future agents.

## Impact

- Local build tooling, Docker Hub push access, and SSH access to the existing
  CVM are required for a real release.
- The CVM remains responsible only for pulling digest-addressed images,
  migration, health verification, release recording, rollback and bounded
  image cleanup. It never builds application source.
