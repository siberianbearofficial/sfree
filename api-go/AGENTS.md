When modifying, adding, or removing API endpoints, update the Swagger documentation by editing Go code comments and running `make docs` to regenerate `internal/docs/docs.go`. The `/api/openapi.json`, `/api/docs`, and legacy `/swagger` documentation routes serve that generated document. Run `make docs-check` to verify the committed generated docs are fresh. Do not commit generated artifacts such as `swagger.yaml` or `swagger.json`; `docs.go` is the single source of truth.

Run `golangci-lint run` and `make test` for this module. Skip integration tests; unit tests are sufficient.
