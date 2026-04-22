# Security and Dependency Audit

Date: 2026-04-22

Scope: `webui` npm dependency baseline and Woodpecker frontend validation.

## Findings

The committed `webui/package-lock.json` has no high or critical findings from
`npm audit --audit-level=high`.

## Changes Made

- Added `npm audit --audit-level=high` to `.woodpecker/webui.yml` after
  lockfile-based install and before lint/build validation.
- Updated `docs/ci.md` so the webui pipeline and dependency audit notes describe
  the final high threshold.

## Validation

- Ran `npm audit --audit-level=high` in `webui`; it reported zero
  vulnerabilities.
- Woodpecker remains responsible for the required lint, build, Playwright, and
  PR audit gates.
