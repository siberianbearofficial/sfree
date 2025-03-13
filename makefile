all: help

SRC = .

.phony: help
help:
	@echo "commands: install-deps install-deps-stdin install-dev-deps ruff black mypy pytest pytest-docker lint clean"

.phony: install-deps
install-deps:
	@bash install_requirements.sh

.phony: install-deps-stdin
install-deps-stdin:
	@bash install_requirements.sh --stdin

.phony: install-dev-deps
install-dev-deps:
	@pip install -r dev.requirements.txt

.phony: ruff
ruff:
	@ruff check $(SRC)

.phony: black
black:
	@black $(SRC) --check

.phony: mypy
mypy:
	@mypy $(SRC)

.phony: test
pytest:
	@pytest tests/

.phony: test-docker
test-docker:
	@bash pytest_docker.sh

.phony: lint
lint: ruff black mypy

.phony: run-environment
run-environment:
	docker compose up -d

.phony: stop-environment
stop-environment:
	docker compose down

.phony: clean
clean:
	find . -type d -name "__pycache__" -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete
	find . -type d -name ".mypy_cache" -exec rm -rf {} +
