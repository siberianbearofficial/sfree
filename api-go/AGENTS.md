When modifying, adding, or removing API endpoints, update the Swagger documentation by editing Go code comments and regenerating `internal/docs/docs.go`. Do not commit generated artifacts such as `swagger.yaml` or `swagger.json`; `docs.go` is the single source of truth.

Run `golangci-lint run` and `make test` for this module. Skip integration tests; unit tests are sufficient.
