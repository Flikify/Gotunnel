#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

normalize_version() {
    local value="$1"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    if [[ -z "$value" ]]; then
        echo "v0.0.0-dev"
        return
    fi
    if [[ "$value" =~ ^[vV] ]]; then
        echo "$value"
        return
    fi
    if [[ "$value" =~ ^[0-9]+(\.[0-9]+){1,3}([-.+].*)?$ ]]; then
        echo "v$value"
        return
    fi
    echo "$value"
}

if [[ -n "${VERSION:-}" ]]; then
    normalize_version "${VERSION}"
    exit 0
fi

if [[ "${GITHUB_REF_TYPE:-}" == "tag" && -n "${GITHUB_REF_NAME:-}" ]]; then
    normalize_version "${GITHUB_REF_NAME}"
    exit 0
fi

if exact_tag="$(git -C "$ROOT_DIR" describe --tags --exact-match 2>/dev/null)"; then
    normalize_version "$exact_tag"
    exit 0
fi

git_commit="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || true)"
latest_tag="$(git -C "$ROOT_DIR" describe --tags --abbrev=0 2>/dev/null || true)"

if [[ -n "$latest_tag" ]]; then
    latest_tag="$(normalize_version "$latest_tag")"
    if [[ -n "$git_commit" ]]; then
        echo "${latest_tag}-dev+${git_commit}"
    else
        echo "${latest_tag}-dev"
    fi
    exit 0
fi

if [[ -n "$git_commit" ]]; then
    echo "v0.0.0-dev+${git_commit}"
else
    echo "v0.0.0-dev"
fi
