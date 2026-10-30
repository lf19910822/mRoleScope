# Tasks

## 1. Contract regressions

- [x] 1.1 Add title-only V4 regressions proving bare `AI` and `Agent` no
  longer create `platform_infra` evidence, while explicit platform aliases
  and reviewed AI-application aliases remain classified.

## 2. Dictionary and documentation

- [x] 2.1 Add immutable V4, retain V1/V2/V3 assets, and switch the embedded
  current asset to V4 without changing parser selection logic.
- [x] 2.2 Maintain the complete reviewed V4 dictionary in
  `Docs/开发指导/JD建模/岗位族词典_latest.md`.

## 3. Verification and delivery

- [x] 3.1 Run focused parser tests, `make test`, generated-code hygiene,
  strict OpenSpec validation, and diff checks.
- [ ] 3.2 Merge V4, build/push images, and deploy through GitHub Actions.
- [ ] 3.3 Run only the provider-free pending analysis on CVM and record
  read-only V4 aggregate results.
