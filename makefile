all: help

.phony: help install-deps install-deps-stdin install-dev-deps ruff black mypy pytest pytest-docker lint clean

SRC = .

help:
	@echo "commands: install-deps install-deps-stdin install-dev-deps ruff black mypy pytest pytest-docker lint clean"

install-deps:
	@bash install_requirements.sh

install-deps-stdin:
	@bash install_requirements.sh --stdin

install-dev-deps:
	@pip install -r dev.requirements.txt

ruff:
	@ruff check $(SRC)

black:
	@black $(SRC) --check

mypy:
	@mypy $(SRC)

pytest:
	docker compose up -d
	@pytest tests/
	docker compose down

pytest-docker:
	@bash pytest_docker.sh

lint: ruff black mypy

clean:
	find . -type d -name "__pycache__" -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete
	find . -type d -name ".mypy_cache" -exec rm -rf {} +
