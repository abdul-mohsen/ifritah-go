#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f VERSION ]]; then
	echo "VERSION file is missing" >&2
	exit 1
fi

version="$(tr -d '[:space:]' < VERSION)"
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "VERSION must match ^v[0-9]+\\.[0-9]+\\.[0-9]+$; got '$version'" >&2
	exit 1
fi

repo="${GITHUB_REPOSITORY:-}"
api_url="${GITHUB_API_URL:-https://api.github.com}"
token="${GITHUB_TOKEN:-}"

if [[ -z "$repo" ]]; then
	echo "GITHUB_REPOSITORY is required to compare VERSION with the latest release" >&2
	exit 1
fi

headers=(
	-H "Accept: application/vnd.github+json"
	-H "X-GitHub-Api-Version: 2022-11-28"
)
if [[ -n "$token" ]]; then
	headers+=(-H "Authorization: Bearer ${token}")
fi

response_file="$(mktemp)"
trap 'rm -f "$response_file"' EXIT

if ! status="$(curl --connect-timeout 10 --max-time 30 --retry 3 -sS -o "$response_file" -w "%{http_code}" "${headers[@]}" "${api_url}/repos/${repo}/releases/latest")"; then
	echo "failed to fetch latest GitHub Release" >&2
	exit 1
fi

latest=""
case "$status" in
	200)
		latest="$(python3 - "$response_file" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as release_file:
    print(json.load(release_file).get("tag_name", ""))
PY
		)"
		if [[ ! "$latest" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
			echo "latest GitHub Release tag must be SemVer; got '$latest'" >&2
			exit 1
		fi
		;;
	404)
		echo "No GitHub Releases found; allowing initial VERSION ${version}."
		;;
	*)
		echo "failed to fetch latest GitHub Release: HTTP ${status}" >&2
		cat "$response_file" >&2
		exit 1
		;;
esac

if [[ -n "$latest" && "$version" != "$latest" ]]; then
	current_number="${version#v}"
	latest_number="${latest#v}"
	lowest="$(printf '%s\n%s\n' "$current_number" "$latest_number" | sort -V | head -n1)"
	if [[ "$lowest" == "$current_number" ]]; then
		echo "VERSION ${version} is lower than latest GitHub Release ${latest}" >&2
		exit 1
	fi
fi

short_sha="${GITHUB_SHA:-$(git rev-parse HEAD)}"
short_sha="${short_sha:0:7}"

echo "VERSION ${version} is valid."
if [[ -n "$latest" ]]; then
	echo "Latest GitHub Release: ${latest}."
fi

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
	{
		echo "version=${version}"
		echo "latest_release=${latest}"
		echo "short_sha=${short_sha}"
	} >> "$GITHUB_OUTPUT"
fi
