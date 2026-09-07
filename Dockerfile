# =============================================================================
# ifritah-go backend image — multi-stage Go build
# =============================================================================
# Build:  docker build -t youruser/ifritah-api:dev .
# Run:    docker run --env-file .env -p 8090:8090 youruser/ifritah-api:dev
# =============================================================================

# ---- Build stage ----
ARG APP_VERSION=v0.0.1
ARG APP_COMMIT=unknown
ARG APP_BUILD_CHANNEL=dev
ARG APP_BUILD_SOURCE=
ARG APP_BUILT_AT=
ARG APP_BUILD_WORKFLOW_RUN=
ARG APP_BUILD_WORKFLOW_URL=
ARG APP_COMMIT_SHORT=
ARG APP_IMAGE_REF=
ARG APP_IMAGE_DIGEST=
FROM golang:1.25-bookworm AS builder
ARG APP_VERSION
ARG APP_COMMIT
ARG APP_BUILD_CHANNEL
ARG APP_BUILD_SOURCE
ARG APP_BUILT_AT
ARG APP_BUILD_WORKFLOW_RUN
ARG APP_BUILD_WORKFLOW_URL
ARG APP_COMMIT_SHORT
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
    case "$APP_BUILD_CHANNEL" in dev|release) ;; *) echo "APP_BUILD_CHANNEL must be dev or release" >&2; exit 1 ;; esac; \
    commit_short="$APP_COMMIT_SHORT"; \
    test -n "$commit_short" && test "$commit_short" = "$(printf '%s' "$APP_COMMIT" | cut -c1-7)" || commit_short="$(printf '%s' "$APP_COMMIT" | cut -c1-7)"; \
    ldflags="-s -w -X ifritah/web-service-gin/pkg/buildinfo.Version=$APP_VERSION -X ifritah/web-service-gin/pkg/buildinfo.Channel=$APP_BUILD_CHANNEL -X ifritah/web-service-gin/pkg/buildinfo.Commit=$APP_COMMIT -X ifritah/web-service-gin/pkg/buildinfo.CommitShort=$commit_short -X ifritah/web-service-gin/pkg/buildinfo.WorkflowRun=$APP_BUILD_WORKFLOW_RUN -X ifritah/web-service-gin/pkg/buildinfo.WorkflowURL=$APP_BUILD_WORKFLOW_URL -X ifritah/web-service-gin/pkg/buildinfo.Source=$APP_BUILD_SOURCE -X ifritah/web-service-gin/pkg/buildinfo.BuiltAt=$APP_BUILT_AT -X ifritah/web-service-gin/pkg/buildinfo.ImageRef=$APP_IMAGE_REF -X ifritah/web-service-gin/pkg/buildinfo.ImageDigest=$APP_IMAGE_DIGEST"; \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="$ldflags" -o /out/ifritah .

# ---- Runtime stage ----
FROM alpine:3.20
ARG APP_VERSION
ARG APP_COMMIT
ARG APP_BUILD_CHANNEL
ARG APP_BUILD_SOURCE
ARG APP_BUILT_AT
ARG APP_BUILD_WORKFLOW_RUN
ARG APP_BUILD_WORKFLOW_URL
ARG APP_COMMIT_SHORT
ARG APP_IMAGE_REF
ARG APP_IMAGE_DIGEST

ENV APP_VERSION=${APP_VERSION}
ENV APP_COMMIT=${APP_COMMIT}
ENV APP_COMMIT_SHORT=${APP_COMMIT_SHORT}
ENV APP_BUILD_CHANNEL=${APP_BUILD_CHANNEL}
ENV APP_BUILD_SOURCE=${APP_BUILD_SOURCE}
ENV APP_BUILT_AT=${APP_BUILT_AT}
ENV APP_BUILD_WORKFLOW_RUN=${APP_BUILD_WORKFLOW_RUN}
ENV APP_BUILD_WORKFLOW_URL=${APP_BUILD_WORKFLOW_URL}
ENV APP_IMAGE_REF=${APP_IMAGE_REF}
ENV APP_IMAGE_DIGEST=${APP_IMAGE_DIGEST}
ENV APP_CHANNEL=${APP_BUILD_CHANNEL}
ENV APP_SOURCE=${APP_BUILD_SOURCE}
ENV APP_CREATED=${APP_BUILT_AT}
ENV APP_WORKFLOW_RUN=${APP_BUILD_WORKFLOW_RUN}
LABEL org.opencontainers.image.version=${APP_VERSION}
LABEL org.opencontainers.image.revision=${APP_COMMIT}
LABEL org.opencontainers.image.source=${APP_BUILD_SOURCE}
LABEL org.opencontainers.image.created=${APP_BUILT_AT}
LABEL org.opencontainers.image.ref.name=${APP_IMAGE_REF}
LABEL org.opencontainers.image.channel=${APP_BUILD_CHANNEL}
LABEL org.opencontainers.image.workflow.run=${APP_BUILD_WORKFLOW_RUN}
LABEL com.ifritah.build.channel=${APP_BUILD_CHANNEL}
LABEL com.ifritah.build.commit=${APP_COMMIT}
LABEL com.ifritah.build.commit_short=${APP_COMMIT_SHORT}
LABEL com.ifritah.build.workflow_run=${APP_BUILD_WORKFLOW_RUN}
LABEL com.ifritah.build.workflow_url=${APP_BUILD_WORKFLOW_URL}
LABEL com.ifritah.build.image_ref=${APP_IMAGE_REF}
LABEL com.ifritah.build.built_at=${APP_BUILT_AT}
LABEL com.ifritah.build.digest=${APP_IMAGE_DIGEST}

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
