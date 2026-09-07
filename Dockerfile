# =============================================================================
# ifritah-go backend image — multi-stage Go build
# =============================================================================
# Build:  docker build -t youruser/ifritah-api:dev .
# Run:    docker run --env-file .env -p 8090:8090 youruser/ifritah-api:dev
# =============================================================================

# ---- Build stage ----
ARG APP_VERSION=v0.0.1
ARG APP_COMMIT=unknown
ARG APP_CHANNEL=dev
ARG APP_SOURCE=
ARG APP_CREATED=
ARG APP_WORKFLOW_RUN=
ARG APP_IMAGE_REF=
ARG APP_IMAGE_DIGEST=
FROM golang:1.25-bookworm AS builder
ARG APP_VERSION
ARG APP_COMMIT
ARG APP_CHANNEL
ARG APP_SOURCE
ARG APP_CREATED
ARG APP_WORKFLOW_RUN
ARG APP_IMAGE_REF
ARG APP_IMAGE_DIGEST

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
COPY VERSION ./VERSION
RUN sqlc generate
RUN set -eu; \
    test -n "$APP_VERSION"; \
    test "$APP_VERSION" != "v0.0.0"; \
    case "$APP_CHANNEL" in dev|release) ;; *) echo "APP_CHANNEL must be dev or release" >&2; exit 1 ;; esac; \
    commit_short="$(printf '%s' "$APP_COMMIT" | cut -c1-7)"; \
    ldflags="-s -w -X ifritah/web-service-gin/pkg/buildinfo.Version=$APP_VERSION -X ifritah/web-service-gin/pkg/buildinfo.Channel=$APP_CHANNEL -X ifritah/web-service-gin/pkg/buildinfo.Commit=$APP_COMMIT -X ifritah/web-service-gin/pkg/buildinfo.CommitShort=$commit_short -X ifritah/web-service-gin/pkg/buildinfo.WorkflowRun=$APP_WORKFLOW_RUN -X ifritah/web-service-gin/pkg/buildinfo.Source=$APP_SOURCE -X ifritah/web-service-gin/pkg/buildinfo.BuiltAt=$APP_CREATED -X ifritah/web-service-gin/pkg/buildinfo.ImageRef=$APP_IMAGE_REF -X ifritah/web-service-gin/pkg/buildinfo.ImageDigest=$APP_IMAGE_DIGEST"; \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="$ldflags" -o /out/ifritah .

# ---- Runtime stage ----
FROM alpine:3.20
ARG APP_VERSION
ARG APP_COMMIT
ARG APP_CHANNEL
ARG APP_SOURCE
ARG APP_CREATED
ARG APP_WORKFLOW_RUN
ARG APP_IMAGE_REF
ARG APP_IMAGE_DIGEST

ENV APP_VERSION=${APP_VERSION}
ENV APP_COMMIT=${APP_COMMIT}
ENV APP_CHANNEL=${APP_CHANNEL}
ENV APP_SOURCE=${APP_SOURCE}
ENV APP_CREATED=${APP_CREATED}
ENV APP_WORKFLOW_RUN=${APP_WORKFLOW_RUN}
ENV APP_IMAGE_REF=${APP_IMAGE_REF}
ENV APP_IMAGE_DIGEST=${APP_IMAGE_DIGEST}
LABEL org.opencontainers.image.version=${APP_VERSION}
LABEL org.opencontainers.image.revision=${APP_COMMIT}
LABEL org.opencontainers.image.source=${APP_SOURCE}
LABEL org.opencontainers.image.created=${APP_CREATED}

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
# Use GET (wget -O-) not HEAD (wget --spider): Gin only registers HEAD automatically
# for routes that also have an explicit HEAD handler; our /healthz uses GET only.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://localhost:${SERVER_PORT}/healthz" | grep -q "ok" || exit 1

ENTRYPOINT ["/app/ifritah"]
