.PHONY: build run test test-unit test-integration test-e2e lint docker-build test-e2e-local test-e2e-python

build:
	go build -o bin/server ./cmd/server

run: build
	./bin/server


test: test-unit test-integration test-e2e


test-unit:
	go test ./...


test-integration:
	ENV=test go test -tags=integration ./...


test-e2e:
	ENV=test go test -tags=e2e ./...


test-e2e-python:
	cd .. && python3 -m venv .venv-e2e && . .venv-e2e/bin/activate && \
	pip install -r api-go/e2e/requirements.txt && \
	cd api-go/e2e && PYTHONPATH=.. pytest -q test_api_e2e.py


test-e2e-local:
	docker compose -f docker-compose.e2e.yml up --build --abort-on-container-exit --exit-code-from e2e-tests


lint:
	golangci-lint run


docker-build:
	docker build -t api-go:latest .
