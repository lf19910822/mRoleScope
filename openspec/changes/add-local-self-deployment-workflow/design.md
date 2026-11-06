# Design

## Release chain

`scripts/release-remote-development.sh --deploy` is the only normal release
entry point. It fails closed unless the checkout is clean, currently on
`main`, and exactly matches a freshly fetched `origin/main` revision. It then
runs the local verification target, builds the four Linux/amd64 images with
the full Git revision in `org.opencontainers.image.revision`, pushes them to
the configured Docker Hub namespace, and resolves each pushed immutable
digest from Docker's `RepoDigests` metadata.

The command validates the local revision label before each push. The existing
CVM helper independently pulls each digest and validates all four labels
again. This makes a tag only a delivery convenience: the CVM deployment input
is always four immutable `docker.io/...@sha256:...` references.

## CVM boundary

The local command opens one non-interactive SSH session to the existing CVM.
It downloads the root bootstrap asset only from the immutable Git revision,
checks its SHA-256 against the local reviewed file, refreshes the fixed
root-owned deployment assets, then calls:

```bash
sudo -u mrolescope-deploy sudo -n /usr/local/sbin/mrolescope-deploy \
  <api@digest> <migrate@digest> <worker@digest> <web-edge@digest>
```

The helper retains its namespace allowlist, revision validation, migration,
health checks, rollback and two-release image retention. The release command
does not read runtime configuration, passwords, Docker credentials, database
URLs, or private keys.

## Failure boundary

Verification, local build, push, bootstrap integrity, SSH, or deployment
failure stops the command immediately. It never retries a failed deployment
blindly and never removes Docker volumes, arbitrary images, containers, or
local worktree files. A failed push cannot reach the CVM; a failed CVM
deployment is left to the root helper's existing record and rollback logic.

## Non-goals

- Replacing GitHub as the reviewed source repository.
- Building application images, exposing new ports, or storing Docker Hub
  credentials on the CVM.
- Enabling the Web Edge public profile; its separate manual release gate
  remains authoritative.
