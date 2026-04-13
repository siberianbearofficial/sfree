FROM python:3.10-slim AS base

WORKDIR /app

FROM base AS dependencies

COPY requirements.txt .
COPY ./utils/install_requirements.sh .
COPY ./utils/pytest_docker.sh .
COPY ./makefile .

RUN apt-get update && apt-get install make

RUN echo "/app/dependencies" | make install-deps-stdin

ENV PATH=/app/dependencies/bin:$PATH \
    PYTHONPATH=/app/dependencies:$PYTHONPATH

FROM dependencies AS linters

COPY . .

RUN make install-dev-deps

FROM base AS runtime

COPY --from=dependencies /app/dependencies /app/.local

ENV PATH=/app/.local/bin:$PATH \
    PYTHONPATH=/app/.local:$PYTHONPATH

COPY . .

EXPOSE 3000
