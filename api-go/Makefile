.PHONY: build run test test-unit test-integration test-e2e lint docker-build

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


simple-test-e2e:
	cd ../api && venv/bin/python client.py


lint:
	golangci-lint run


docker-build:
	docker build -t api-go:latest .
