# =============================================================================
# ifritah-go backend image — multi-stage Go build
# =============================================================================
# Build:  docker build -t youruser/ifritah-api:dev .
# Run:    docker run --env-file .env -p 8090:8090 youruser/ifritah-api:dev
# =============================================================================

# ---- Build stage ----
FROM golang:1.25-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# pkg/db/gen is gitignored and produced by `sqlc generate`. Bake the
# generation step into the image so the build does not depend on local
# state — pkg/db/gen is .dockerignore'd because it is .gitignore'd, so
# without this step CI fails with "package .../pkg/db/gen is not in std".
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0

# Copy only the inputs needed to build the binary. Avoids a recursive
# `COPY . .` which can leak local state into the image (Sonar docker:S6470).
COPY main.go ./
COPY pkg ./pkg
COPY fonts ./fonts
COPY sqlc.yaml ./
RUN sqlc generate
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/ifritah .

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/ifritah /app/ifritah
COPY --chown=app:app pkg/db/schema /app/db/schema
COPY --chown=app:app pkg/db/migrations /app/db/migrations
COPY --chown=app:app pkg/db/runtime_migrations /app/db/migrations

RUN mkdir -p /app/uploads /app/data && chown -R app:app /app

USER app

# Document the conventional port; the runtime must still set SERVER_PORT.
EXPOSE 8090

# /healthz returns 200 OK from the Gin router as soon as the server is up.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider "http://localhost:${SERVER_PORT}/healthz" || exit 1

ENTRYPOINT ["/app/ifritah"]
