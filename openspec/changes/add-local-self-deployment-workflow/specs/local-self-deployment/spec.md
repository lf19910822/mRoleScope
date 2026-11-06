## ADDED Requirements

### Requirement: A local release starts only from reviewed main

The release command MUST refuse a state-changing release unless the worktree
is clean, the checked-out branch is `main`, and `HEAD` exactly equals a freshly
fetched `origin/main` full 40-character revision.

#### Scenario: A feature checkout attempts release

- **WHEN** an operator runs the release command from a branch other than
  `main`, or from a checkout that differs from `origin/main`
- **THEN** the command MUST stop before running local verification, Docker
  build/push, or SSH

### Requirement: A release deploys four matching immutable images

The local release command MUST verify locally, build and push API, migration,
worker and Web Edge Linux/amd64 images. Each image MUST have the same source
revision OCI label as the reviewed main revision, and the CVM input MUST be
four Docker Hub digest references rather than tags.

#### Scenario: A normal release reaches the CVM

- **WHEN** all local checks and image pushes succeed
- **THEN** the command MUST pass exactly one immutable digest for each of API,
  migration, worker and Web Edge to the CVM restricted deployment helper

#### Scenario: A built image has a mismatched revision label

- **WHEN** any locally built image does not contain the expected
  `org.opencontainers.image.revision` label
- **THEN** the command MUST stop before pushing that image or contacting the
  CVM

### Requirement: The CVM retains the existing deployment boundary

Before invoking the existing root-owned deployment helper, the command MUST
refresh the reviewed deployment assets from the same immutable source revision
and validate the downloaded bootstrap file against the local SHA-256. The
command MUST use the existing `mrolescope-deploy` service account invocation;
it MUST NOT build application source or read runtime secrets on the CVM.

#### Scenario: The downloaded bootstrap differs from the reviewed file

- **WHEN** the CVM-download bootstrap SHA-256 does not match the local
  reviewed bootstrap asset
- **THEN** the remote command MUST stop without refreshing assets or deploying
  images
