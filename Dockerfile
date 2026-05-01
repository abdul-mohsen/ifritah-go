# =============================================================================
# ifritah-go backend image — multi-stage Go build
# =============================================================================
# Build:  docker build -t youruser/ifritah-api:dev .
# Run:    docker run --env-file .env -p 8090:8090 youruser/ifritah-api:dev
# =============================================================================

# ---- Build stage ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# Copy only the inputs needed to build the binary. Avoids a recursive
# `COPY . .` which can leak local state into the image (Sonar docker:S6470).
COPY main.go ./
COPY pkg ./pkg
COPY fonts ./fonts
COPY sqlc.yaml ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/ifritah .

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/ifritah /app/ifritah

RUN mkdir -p /app/uploads /app/data && chown -R app:app /app

USER app

# Document the conventional port; the runtime must still set SERVER_PORT.
EXPOSE 8090

# /healthz returns 200 OK from the Gin router as soon as the server is up.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider "http://localhost:${SERVER_PORT}/healthz" || exit 1

ENTRYPOINT ["/app/ifritah"]
