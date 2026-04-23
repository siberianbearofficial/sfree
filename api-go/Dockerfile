# syntax=docker/dockerfile:1
ARG GO_IMAGE=golang:1.25
FROM ${GO_IMAGE} AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/server ./server
COPY --from=builder /app/config ./config
EXPOSE 8080
ENTRYPOINT ["/app/server"]
