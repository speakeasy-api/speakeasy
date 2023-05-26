#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

(
  cd "${SCRIPT_DIR}/.."
  export GOPRIVATE=github.com/speakeasy-api/*
  read -r CURRENT_VERSION <<<$(git describe --tags | awk '{print substr($1,2); }')
  echo "  => CURRENT_VERSION=${CURRENT_VERSION}"

  read -r CURRENT_OPENAPI_GENERATION_VERSION <<<$(cat go.mod | grep github.com/speakeasy-api/openapi-generation/v2 | awk '{ print $2 }' | awk '{print substr($1,2); }')
  echo "  => CURRENT_OPENAPI_GENERATION_VERSION=${CURRENT_OPENAPI_GENERATION_VERSION}"

  read -r NEXT_OPENAPI_GENERATION_VERSION <<<$(go list -m -versions github.com/speakeasy-api/openapi-generation/v2 | awk '{print $(NF)}' | awk '{print substr($1,2); }')
  echo "  => NEXT_OPENAPI_GENERATION_VERSION=${NEXT_OPENAPI_GENERATION_VERSION}"

  read -r SEMVER_CHANGE <<<$("${SCRIPT_DIR}/semver.bash" diff "${CURRENT_OPENAPI_GENERATION_VERSION}" "${NEXT_OPENAPI_GENERATION_VERSION}")
  echo "  => SEMVER_CHANGE=${SEMVER_CHANGE}"

  if [[ "${SEMVER_CHANGE}" == "major" ]]; then
    echo "  => MAJOR SEMVER_CHANGE detected in downstream library. Downgrading it to minor as we're wrapped in a CLI"
    SEMVER_CHANGE=minor
  fi

  read -r BUMPED_CURRENT_VERSION <<<$("${SCRIPT_DIR}/semver.bash" bump "${SEMVER_CHANGE}" "${CURRENT_VERSION}")
  echo "  => BUMPED_CURRENT_VERSION=${BUMPED_CURRENT_VERSION}"

  read -p "About to bump $SEMVER_CHANGE from $CURRENT_VERSION to $BUMPED_CURRENT_VERSION. Confirm (y/N): " choice
  case "$choice" in
    y );;
    * ) exit 1;;
  esac

  echo "$ git add -A"
  git add -A
  echo "$ git commit -m"chore: bump openapi-generation/v2 to ${NEXT_OPENAPI_GENERATION_VERSION}""
  git commit -m"chore: bump openapi-generation/v2 to ${NEXT_OPENAPI_GENERATION_VERSION}"
  echo "$ git tag \"v${BUMPED_CURRENT_VERSION}\""
  git tag "v${BUMPED_CURRENT_VERSION}"
  echo "===== When you are ready, execute the following command to upgrade ====="
  echo "$ git push origin main --tags"
)
