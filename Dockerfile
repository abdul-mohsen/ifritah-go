# =============================================================================
# ifritah-go backend image — multi-stage Go build
# =============================================================================
# Build:  CI supplies required APP_* provenance args; local image builds must
#         pass the same semantic version, commit, tag, workflow, and ref values.
# Run:    docker run --env-file .env -p 8090:8090 youruser/ifritah-api:dev
# =============================================================================

# ---- Build stage ----
ARG APP_VERSION=
ARG APP_COMMIT=
ARG APP_IMAGE_VERSION=${APP_VERSION}
ARG APP_IMAGE_COMMIT=${APP_COMMIT}
ARG APP_BUILD_CHANNEL=dev
ARG APP_IMAGE_CHANNEL=${APP_BUILD_CHANNEL}
ARG APP_IMAGE_TAG=
ARG APP_BUILD_SOURCE=
ARG APP_BUILT_AT=
ARG APP_BUILD_WORKFLOW_RUN=
ARG APP_BUILD_WORKFLOW_URL=
ARG APP_WORKFLOW_RUN_ID=${APP_BUILD_WORKFLOW_RUN}
ARG APP_WORKFLOW_RUN_URL=${APP_BUILD_WORKFLOW_URL}
ARG APP_COMMIT_SHORT=
ARG APP_IMAGE_REF=
FROM golang:1.25-bookworm AS builder
ARG APP_VERSION
ARG APP_COMMIT
ARG APP_IMAGE_VERSION
ARG APP_IMAGE_COMMIT
ARG APP_BUILD_CHANNEL
ARG APP_IMAGE_CHANNEL
ARG APP_IMAGE_TAG
ARG APP_BUILD_SOURCE
ARG APP_BUILT_AT
ARG APP_BUILD_WORKFLOW_RUN
ARG APP_BUILD_WORKFLOW_URL
ARG APP_WORKFLOW_RUN_ID
ARG APP_WORKFLOW_RUN_URL
ARG APP_COMMIT_SHORT
ARG APP_IMAGE_REF

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# pkg/db/gen is gitignored and produced by `sqlc generate`. Copy the
# version-pinned, standalone sqlc binary instead of resolving an
# untracked dependency graph during the application build.
COPY --from=sqlc/sqlc:1.27.0 /workspace/sqlc /usr/local/bin/sqlc

# Copy only the inputs needed to build the binary. Avoids a recursive
# `COPY . .` which can leak local state into the image (Sonar docker:S6470).
COPY main.go ./
COPY pkg ./pkg
COPY fonts ./fonts
COPY sqlc.yaml ./
COPY VERSION ./VERSION
RUN sqlc generate
RUN set -eu; \
    image_version="${APP_IMAGE_VERSION:-$APP_VERSION}"; \
    image_commit="${APP_IMAGE_COMMIT:-$APP_COMMIT}"; \
    image_channel="${APP_IMAGE_CHANNEL:-$APP_BUILD_CHANNEL}"; \
    image_tag="${APP_IMAGE_TAG:-}"; \
    workflow_run_id="${APP_WORKFLOW_RUN_ID:-$APP_BUILD_WORKFLOW_RUN}"; \
    workflow_run_url="${APP_WORKFLOW_RUN_URL:-$APP_BUILD_WORKFLOW_URL}"; \
    image_built_at="${APP_BUILT_AT:-$APP_BUILT_AT}"; \
    test -n "$image_version"; \
    test "$image_version" != "v0.0.0"; \
    printf '%s' "$image_version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; \
    test -n "$image_commit"; \
    printf '%s' "$image_commit" | grep -Eq '^[0-9a-f]{40}$'; \
    case "$image_channel" in dev|release) ;; *) echo "APP_IMAGE_CHANNEL must be dev or release" >&2; exit 1 ;; esac; \
    if [ -z "$image_tag" ]; then \
        if [ "$image_channel" = "dev" ]; then \
            image_tag="dev"; \
        else \
            image_tag="$image_version"; \
        fi; \
    fi; \
    test -n "$workflow_run_id"; \
    printf '%s' "$workflow_run_id" | grep -Eq '^[0-9]+$'; \
    test -n "$workflow_run_url"; \
    commit_short="$APP_COMMIT_SHORT"; \
    test -n "$commit_short" && test "$commit_short" = "$(printf '%s' "$image_commit" | cut -c1-7)" || commit_short="$(printf '%s' "$image_commit" | cut -c1-7)"; \
    ldflags="-s -w"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.Version=$image_version"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.Channel=$image_channel"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.Commit=$image_commit"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.CommitShort=$commit_short"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.ImageTag=$image_tag"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.WorkflowRunID=$workflow_run_id"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.WorkflowRun=$workflow_run_id"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.WorkflowURL=$workflow_run_url"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.Source=$APP_BUILD_SOURCE"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.BuiltAt=$image_built_at"; \
    ldflags="$ldflags -X ifritah/web-service-gin/pkg/buildinfo.ImageRef=$APP_IMAGE_REF"; \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="$ldflags" -o /out/ifritah .

# ---- Runtime stage ----
FROM alpine:3.20
ARG APP_VERSION
ARG APP_COMMIT
ARG APP_IMAGE_VERSION
ARG APP_IMAGE_COMMIT
ARG APP_BUILD_CHANNEL
ARG APP_IMAGE_CHANNEL
ARG APP_IMAGE_TAG
ARG APP_BUILD_SOURCE
ARG APP_BUILT_AT
ARG APP_BUILD_WORKFLOW_RUN
ARG APP_BUILD_WORKFLOW_URL
ARG APP_WORKFLOW_RUN_ID
ARG APP_WORKFLOW_RUN_URL
ARG APP_COMMIT_SHORT
ARG APP_IMAGE_REF

ENV APP_VERSION=${APP_IMAGE_VERSION}
ENV APP_COMMIT=${APP_IMAGE_COMMIT}
ENV APP_IMAGE_VERSION=${APP_IMAGE_VERSION}
ENV APP_IMAGE_COMMIT=${APP_IMAGE_COMMIT}
ENV APP_IMAGE_CHANNEL=${APP_IMAGE_CHANNEL}
ENV APP_IMAGE_TAG=${APP_IMAGE_TAG}
ENV APP_COMMIT_SHORT=${APP_COMMIT_SHORT}
ENV APP_BUILD_CHANNEL=${APP_IMAGE_CHANNEL}
ENV APP_BUILD_SOURCE=${APP_BUILD_SOURCE}
ENV APP_BUILT_AT=${APP_BUILT_AT}
ENV APP_BUILD_WORKFLOW_RUN=${APP_WORKFLOW_RUN_ID}
ENV APP_BUILD_WORKFLOW_URL=${APP_WORKFLOW_RUN_URL}
ENV APP_WORKFLOW_RUN_ID=${APP_WORKFLOW_RUN_ID}
ENV APP_WORKFLOW_RUN_URL=${APP_WORKFLOW_RUN_URL}
ENV APP_IMAGE_REF=${APP_IMAGE_REF}
# The pushed manifest digest is only known after build/push, so deployment
# tooling must inject APP_IMAGE_DIGEST/APP_DIGEST at runtime when it pins by digest.
ENV APP_IMAGE_DIGEST=
ENV APP_CHANNEL=${APP_IMAGE_CHANNEL}
ENV APP_TAG=${APP_IMAGE_TAG}
ENV APP_SOURCE=${APP_BUILD_SOURCE}
ENV APP_CREATED=${APP_BUILT_AT}
ENV APP_WORKFLOW_RUN=${APP_WORKFLOW_RUN_ID}
ENV APP_REF=${APP_IMAGE_REF}
ENV APP_DIGEST=
LABEL org.opencontainers.image.version=${APP_IMAGE_VERSION}
LABEL org.opencontainers.image.revision=${APP_IMAGE_COMMIT}
LABEL org.opencontainers.image.source=${APP_BUILD_SOURCE}
LABEL org.opencontainers.image.created=${APP_BUILT_AT}
LABEL org.opencontainers.image.ref.name=${APP_IMAGE_TAG}
LABEL org.opencontainers.image.channel=${APP_IMAGE_CHANNEL}
LABEL org.opencontainers.image.workflow.run=${APP_WORKFLOW_RUN_ID}
LABEL com.ifritah.build.channel=${APP_IMAGE_CHANNEL}
LABEL com.ifritah.build.version=${APP_IMAGE_VERSION}
LABEL com.ifritah.build.commit=${APP_IMAGE_COMMIT}
LABEL com.ifritah.build.workflow_run_id=${APP_WORKFLOW_RUN_ID}
LABEL com.ifritah.build.workflow_run_url=${APP_WORKFLOW_RUN_URL}
LABEL com.ifritah.build.commit_short=${APP_COMMIT_SHORT}
LABEL com.ifritah.build.tag=${APP_IMAGE_TAG}
LABEL com.ifritah.build.ref=${APP_IMAGE_REF}
LABEL com.ifritah.build.workflow_url=${APP_WORKFLOW_RUN_URL}
LABEL com.ifritah.build.image_ref=${APP_IMAGE_REF}
LABEL com.ifritah.build.built_at=${APP_BUILT_AT}

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
