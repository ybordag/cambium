# syntax=docker/dockerfile:1
# Multi-stage build: compile on golang builder, run on minimal distroless image.
# Build for linux/amd64 (DGX Spark). On Apple Silicon:
#   docker buildx build --platform linux/amd64 -t ghcr.io/ybordag/cambium:latest .

FROM --platform=linux/arm64 golang:1.25-alpine AS builder
WORKDIR /app

# Download dependencies first (cached separately from source)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 produces a fully static binary with no libc dependency
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o cambium ./cmd/server/

# Distroless: no shell, no package manager, minimal attack surface (~2MB OS)
FROM --platform=linux/arm64 gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app/cambium /cambium
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/cambium"]
