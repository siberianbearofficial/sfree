# Security and Dependency Audit

Date: 2026-04-22

Scope: `webui` npm dependency baseline and Woodpecker frontend validation.

## Findings

The committed `webui/package-lock.json` has no high or critical findings from
`npm audit --audit-level=high`.

## Changes Made

- Updated the webui tooling dependency baseline within existing major versions
  so the lockfile resolves fixed transitive versions for the high-severity audit
  findings in `vite`, `rollup`, `tar`, `picomatch`, `minimatch`, and `flatted`.
- Added narrow npm overrides for `rollup` and `flatted` because those
  transitives remained below fixed versions after direct tooling upgrades.
- Added `npm audit --audit-level=high` to `.woodpecker/webui.yml` after
  lockfile-based install and before lint/build validation.
- Updated `docs/ci.md` so the webui pipeline and dependency audit notes describe
  the final high threshold.

## Validation

- Ran `npm audit --audit-level=high` in `webui`; it reported zero
  vulnerabilities.
- Ran `npx -y npm@10.8.2 ci --include=dev` followed by
  `npx -y npm@10.8.2 audit --audit-level=high`; both completed with zero
  vulnerabilities.
- Woodpecker remains responsible for the required lint, build, Playwright, and
  PR audit gates.
