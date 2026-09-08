#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
dockerfile="$root/Dockerfile"
workflow="$root/.github/workflows/deploy.yml"
main="$root/main.go"

require_text() {
	local file="$1" text="$2"
	if ! grep -Fq -- "$text" "$file"; then
		echo "$file is missing required build-contract text: $text" >&2
		exit 1
	fi
}

for env_name in \
	APP_VERSION APP_COMMIT APP_COMMIT_SHORT APP_BUILD_CHANNEL \
	APP_BUILD_SOURCE APP_BUILD_WORKFLOW_RUN APP_BUILD_WORKFLOW_URL APP_BUILT_AT \
	APP_WORKFLOW_RUN_ID APP_WORKFLOW_RUN_URL APP_IMAGE_VERSION APP_IMAGE_COMMIT \
	APP_IMAGE_CHANNEL APP_IMAGE_TAG APP_IMAGE_REF APP_IMAGE_DIGEST APP_TAG APP_REF APP_DIGEST; do
	require_text "$dockerfile" "ENV ${env_name}="
done

for build_arg in \
	APP_VERSION APP_COMMIT APP_COMMIT_SHORT APP_BUILD_CHANNEL \
	APP_BUILD_SOURCE APP_BUILD_WORKFLOW_RUN APP_BUILD_WORKFLOW_URL APP_BUILT_AT \
	APP_WORKFLOW_RUN_ID APP_WORKFLOW_RUN_URL APP_IMAGE_VERSION APP_IMAGE_COMMIT \
	APP_IMAGE_CHANNEL APP_IMAGE_TAG APP_IMAGE_REF APP_IMAGE_DIGEST; do
	require_text "$workflow" "${build_arg}="
done

for label_name in \
	com.ifritah.build.channel \
	com.ifritah.build.version \
	com.ifritah.build.commit \
	com.ifritah.build.commit_short \
	com.ifritah.build.workflow_run_id \
	com.ifritah.build.workflow_run_url \
	com.ifritah.build.workflow_run \
	com.ifritah.build.workflow_url \
	com.ifritah.build.tag \
	com.ifritah.build.ref \
	com.ifritah.build.image_ref \
	com.ifritah.build.built_at \
	com.ifritah.build.digest \
	org.opencontainers.image.version \
	org.opencontainers.image.revision \
	org.opencontainers.image.source \
	org.opencontainers.image.created \
	org.opencontainers.image.ref.name \
	org.opencontainers.image.channel \
	org.opencontainers.image.workflow.run; do
	require_text "$dockerfile" "$label_name"
	require_text "$workflow" "$label_name"
done

require_text "$workflow" 'echo "workflow_run=${GITHUB_RUN_ID}"'
require_text "$workflow" 'echo "workflow_run_url=${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"'
require_text "$workflow" 'echo "image_tag=${image_tag}"'
require_text "$workflow" 'type=raw,value=${{ github.sha }}'
require_text "$workflow" 'type=raw,value=${{ steps.version.outputs.short_sha }}'
require_text "$workflow" 'DEV_IMAGE: ${{ env.IMAGE_NAME }}:dev'
require_text "$workflow" 'SOURCE_IMAGE: ${{ env.IMAGE_NAME }}@${{ needs.build-and-push.outputs.digest }}'
require_text "$dockerfile" 'ARG APP_VERSION='
require_text "$dockerfile" 'test "$image_version" != "v0.0.0"'
require_text "$dockerfile" "grep -Eq '^[0-9]+$'"
require_text "$dockerfile" '/healthz'
require_text "$main" 'router.GET("/healthz"'
require_text "$main" 'router.GET("/version"'
