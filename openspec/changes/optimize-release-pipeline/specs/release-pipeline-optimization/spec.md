# Spec Delta

## Purpose

This capability keeps ordinary application releases from repeating unchanged
deployment-boundary downloads while preserving the existing immutable and
root-controlled activation guarantees.

## ADDED Requirements

### Requirement: Deployment assets are fingerprinted

The local release SHALL compute one SHA-256 fingerprint over the reviewed
deployment assets from the exact clean `main` revision and pass that fingerprint
to the CVM deployment boundary without including secrets.

#### Scenario: Fingerprint is produced for a release

- **WHEN** a release passes clean-checkout preflight
- **THEN** the release command has a lowercase 64-character asset fingerprint
  derived from the reviewed deployment asset set

### Requirement: Unchanged deployment assets skip bootstrap

The CVM SHALL skip bootstrap downloads and installation when its root-owned
stored asset fingerprint equals the requested fingerprint, while continuing the
normal immutable image deployment and all migration and health checks.

#### Scenario: Application-only release

- **WHEN** the requested asset fingerprint equals the installed fingerprint
- **THEN** no GitHub Raw or jsDelivr deployment-asset download occurs and the
  four digest-addressed images still proceed through deployment

#### Scenario: First or changed deployment boundary

- **WHEN** the stored fingerprint is missing or differs from the requested one
- **THEN** the reviewed bootstrap downloads, verifies, installs all required
  assets, records the new fingerprint, and only then starts image deployment

### Requirement: Optimization preserves release safety

The optimization SHALL fail closed on an invalid fingerprint or failed asset
verification and SHALL preserve digest validation, migration, health checks,
rollback, and bounded cleanup behavior.

#### Scenario: Invalid fingerprint

- **WHEN** the release or bootstrap receives a value that is not exactly 64
  lowercase hexadecimal characters
- **THEN** it stops before changing the deployment boundary or application
  services

#### Scenario: Image deployment after a cache hit

- **WHEN** bootstrap is skipped because the fingerprint matches
- **THEN** the existing restricted four-digest deploy helper remains the only
  activation path and records success only after migration and health checks

### Requirement: Release phases expose elapsed time

The release command SHALL report elapsed duration for preflight, quality gate,
image publication, asset refresh, and remote deployment phases without printing
credentials or connection strings.

#### Scenario: Timing output

- **WHEN** a release completes or fails
- **THEN** its output contains safe phase names and elapsed durations suitable
  for comparing release performance
