#!/usr/bin/env bash
set -euo pipefail

if [[ -e web ]]; then
  echo 'web already exists; refuse to overwrite it.' >&2
  exit 1
fi

# create-vue 3.22.0 supports the project-pinned Node 20.20.2. Newer CLI releases
# require Node 22.18+ and removed the former --no-* feature flags.
# Vue's official generator creates the baseline configuration and lockfile.
npm create vue@3.22.0 web -- --ts --router --vitest --eslint --prettier
# npm 10.8.2 bundled with the current Node image crashes while resolving the
# current Vue dependency graph. npm 11.21.1 supports Node 20.20.2 and is run
# transiently, without changing the developer's globally installed npm.
npx --yes npm@11.21.1 --prefix web install naive-ui echarts --package-lock=true
