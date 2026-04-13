# api/ — Deprecated Python Backend

> **This directory is deprecated.** The active backend is
> [`api-go/`](../api-go/). All new features, bug fixes, and contributor work
> should target `api-go/` unless a GitHub issue explicitly requests legacy
> Python API changes.

This directory contains the original Python implementation of the SFree API.
It is kept in the repository for historical reference only and is not part of
the active development, CI/CD, or Docker Compose stack.

**Do not:**

- Add new endpoints or features here
- Include this service in Docker Compose setups
- Route new contributors to this code

**Instead:** See [CONTRIBUTING.md](../CONTRIBUTING.md) for how to get started
with the active Go backend.
