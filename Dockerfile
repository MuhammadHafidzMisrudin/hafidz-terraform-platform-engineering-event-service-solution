# syntax=docker/dockerfile:1

# ==============================================================================
# Stage 1 — build
# Compiles a small, statically-linked Linux binary. Kept separate so the Go
# toolchain (~300 MB) never ships in the final image.
# ==============================================================================
FROM golang:1.22-alpine AS build

WORKDIR /src

# Download modules first so this layer is cached until go.mod/go.sum change.
# (This service currently has no external deps, but this keeps builds fast if
# any are added later. go.sum* tolerates the file being absent today.)
COPY go.mod go.sum* ./
RUN go mod download

# Build the service.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/event-service .

# ==============================================================================
# Stage 2 — runtime
# Minimal Alpine base: small, but still has a shell + BusyBox wget so the
# HEALTHCHECK below can probe /health without bundling curl.
# ==============================================================================
FROM alpine:3.20

# ca-certificates for outbound TLS; create an unprivileged user to run as.
RUN apk add --no-cache ca-certificates \
 && addgroup -S app \
 && adduser -S -G app -h /app app

WORKDIR /app
COPY --from=build /out/event-service /app/event-service

# ---- Configuration (override at `docker run` with -e) -------------------------
ENV PORT=8080 \
    ENV=dev \
    PROCESSING_DELAY_MS=1000

# Document the port the service listens on. EXPOSE is static; if you change PORT
# at runtime, publish that port explicitly (e.g. `-e PORT=9000 -p 9000:9000`).
EXPOSE 8080

# Drop privileges.
USER app

# Container-level liveness probe against the always-200 /health endpoint.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${PORT}/health" || exit 1

ENTRYPOINT ["/app/event-service"]
