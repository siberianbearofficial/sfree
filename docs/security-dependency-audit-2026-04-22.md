# Security and Dependency Audit

Date: 2026-04-22

Scope: `api-go` vulnerability baseline, backend Go toolchain images, and
Woodpecker backend validation. The archived Python backend was not modified.

## Backend Remediation

- Moved backend CI and build defaults from Go 1.24 to Go 1.25 so standard
  library findings reported for `crypto/x509`, `crypto/tls`, `html/template`,
  `os`, and `net/url` are covered by the newer toolchain.
- Updated reachable vulnerable backend modules:
  `github.com/golang-jwt/jwt/v5` to `v5.2.2`,
  `github.com/gin-contrib/cors` to `v1.6.0`, and
  `go.opentelemetry.io/otel/sdk` to `v1.40.0`.
- Added a blocking Woodpecker `dependency audit` step for `api-go` that installs
  and runs `govulncheck@v1.1.4`.

## Validation

No CPU-heavy local validation was run. Docker, E2E, full test suites, and
`govulncheck` execution are expected to run in Woodpecker for this repository.
