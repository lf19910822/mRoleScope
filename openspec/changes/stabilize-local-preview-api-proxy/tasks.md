# Tasks

## 1. Configuration

- [x] 1.1 Load Vite mode environment values and preserve explicit process-level proxy overrides.
- [x] 1.2 Default development mode to the current loopback API tunnel at `127.0.0.1:20080`.
- [x] 1.3 Update local preview documentation and remove the stale `18080` target.

## 2. Verification

- [x] 2.1 Refresh the local loopback tunnel and confirm the current CVM API responds through it.
- [x] 2.2 Restart 5175 without a proxy environment variable and confirm `/api/v1/jobs` is proxied rather than served as SPA HTML.
- [x] 2.3 Run the repository quality commands and record their results before final delivery.
