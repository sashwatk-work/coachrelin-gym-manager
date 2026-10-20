# syntax=docker/dockerfile:1.7
# Build context: repository root.

FROM golang:1.25-alpine AS builder
WORKDIR /src/backend
RUN apk add --no-cache ca-certificates
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S gym \
    && adduser -S -G gym gym
WORKDIR /app
COPY --from=builder /out/api /app/api
USER gym:gym
EXPOSE 8080
ENV PORT=8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/api"]
