#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

(
  cd "${SCRIPT_DIR}/.."
  export GOPRIVATE=github.com/speakeasy-api/*
  read -r CURRENT_VERSION <<<$(git describe --tags | awk '{print substr($1,2); }')
  # echo "  => CURRENT_VERSION=${CURRENT_VERSION}"

  read -r CURRENT_OPENAPI_GENERATION_VERSION <<<$(cat go.mod | grep github.com/speakeasy-api/openapi-generation/v2 | awk '{ print $2 }' | awk '{print substr($1,2); }')
  # echo "  => CURRENT_OPENAPI_GENERATION_VERSION=${CURRENT_OPENAPI_GENERATION_VERSION}"
  START_DATE=$(gh release view "v${CURRENT_OPENAPI_GENERATION_VERSION}" --repo speakeasy-api/openapi-generation --json createdAt | jq -r '.createdAt')

  if [[ -z $START_DATE ]]; then
    echo "Could not find current version (v${CURRENT_OPENAPI_GENERATION_VERSION}) release"
    exit 1
  fi

  PRS=$(gh pr list --repo speakeasy-api/openapi-generation --state merged --search "merged:>${START_DATE}" --json title,url | jq -r '.[] | .title+"\n > "+.url+"\n"')
#  echo "  => PRS=${PRS}"
  LATEST_OPENAPI_GENERATION_VERSION=$(gh release list --limit 1 --repo speakeasy-api/openapi-generation --json tagName | jq -r '.[0].tagName')
#  echo "  => LATEST_OPENAPI_GENERATION_VERSION=${LATEST_OPENAPI_GENERATION_VERSION}"

  read -r SEMVER_CHANGE <<<$("${SCRIPT_DIR}/semver.bash" diff "${CURRENT_OPENAPI_GENERATION_VERSION}" "${LATEST_OPENAPI_GENERATION_VERSION}")
#  echo "  => SEMVER_CHANGE=${SEMVER_CHANGE}"

  if [[ "$SEMVER_CHANGE" == "none" || -z $SEMVER_CHANGE ]]; then
    echo "  => No SEMVER_CHANGE detected in downstream library. Exiting"
    exit 0
  fi

  read -r BUMPED_CURRENT_VERSION <<<$("${SCRIPT_DIR}/semver.bash" bump "${SEMVER_CHANGE}" "${CURRENT_VERSION}")
#  echo "  => BUMPED_CURRENT_VERSION=${BUMPED_CURRENT_VERSION}"
  echo "  ===== Pull Requests ==== "
  while IFS= read -r PR; do
    echo -e "${PR}"
  done <<< "$PRS"
  echo "  ===== End Pull Requests ==== "
  if command -v gum &> /dev/null; then
    SUMMARY=$(gum input --placeholder "Commit Message (please summarize changes above): ")
  else
    echo "⚠️  Install gum for a better DX: https://github.com/charmbracelet/gum"
    read -p "Commit Message (please summarize changes above): " SUMMARY
  fi

  go get -v "github.com/speakeasy-api/openapi-generation/v2@${LATEST_OPENAPI_GENERATION_VERSION}"
  go mod tidy

  echo "$ git add go.mod go.sum"
  git add go.mod go.sum
  echo "$ git commit -m\"$SUMMARY\""
  git commit -m"$SUMMARY"
  echo "===== When you are ready, execute the following command to upgrade ====="
  echo "$ git push origin main"
)
