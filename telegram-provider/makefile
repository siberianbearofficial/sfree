all: help

SRC = .

.PHONY: help
help:
@echo "commands: install-deps install-dev-deps ruff black mypy test run"

.PHONY: install-deps
install-deps:
@pip install -r requirements.txt

.PHONY: install-dev-deps
install-dev-deps:
@pip install -r dev.requirements.txt

.PHONY: ruff
ruff:
ruff check $(SRC)

.PHONY: black
black:
black $(SRC) --check

.PHONY: mypy
mypy:
mypy $(SRC)

.PHONY: test
test:
pytest tests/

.PHONY: run
run:
uvicorn main:app --reload --port 8000
