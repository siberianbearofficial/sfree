all: help

SRC = .
UTILS = ./utils

.phony: help
help:
	@echo "commands: install-deps install-deps-stdin install-dev-deps ruff black mypy test test-docker lint run-environment stop-environment run stop urev drev grev clean"

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
test: tests
	@pytest tests/

.phony: test-docker
test-docker:
	@bash pytest_docker.sh

.phony: lint
lint: ruff black mypy

.phony: run-environment
run-environment:
	docker compose up -d
	@export $$(cat local.env | xargs); docker compose -f docker-compose-local.yml up -d

.phony: stop-environment
stop-environment:
	docker compose down
	@export $$(cat local.env | xargs); docker compose -f docker-compose-local.yml stop

.phony: run
run:
	@export $$(cat local.env | xargs); uvicorn main:app --reload

.phony: urev
urev:
	@export $$(cat local.env | xargs); alembic -c migrations/alembic.ini upgrade head

.phony: drev
drev:
	@export $$(cat local.env | xargs); alembic -c migrations/alembic.ini downgrade $(REV)

.phony: grev
grev:
	@export $$(cat local.env | xargs); alembic -c migrations/alembic.ini revision --autogenerate -m $(REV)


.phony: clean
clean:
	find . -type d -name "__pycache__" -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete
	find . -type d -name ".mypy_cache" -exec rm -rf {} +
