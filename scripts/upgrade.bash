#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

(
  cd "${SCRIPT_DIR}/.."
  export GOPRIVATE="github.com/speakeasy-api/*"

  MODULE_PATH="github.com/speakeasy-api/openapi-generation/v2"
  OPENAPI_GENERATION_REPOSITORY="speakeasy-api/openapi-generation"

  MODULE_JSON=$(go mod edit -json)
  CURRENT_OPENAPI_GENERATION_VERSION=$(jq -r --arg module "$MODULE_PATH" '.Require[] | select(.Path == $module) | .Version' <<<"$MODULE_JSON")

  if [[ -z "$CURRENT_OPENAPI_GENERATION_VERSION" || "$CURRENT_OPENAPI_GENERATION_VERSION" == "null" ]]; then
    echo "Could not find required version for ${MODULE_PATH}"
    exit 1
  fi


  CURRENT_OPENAPI_GENERATION_VERSION="${CURRENT_OPENAPI_GENERATION_VERSION#v}"
  START_DATE=$(gh release view "v${CURRENT_OPENAPI_GENERATION_VERSION}" --repo "$OPENAPI_GENERATION_REPOSITORY" --json createdAt | jq -r '.createdAt')

  if [[ -z "$START_DATE" || "$START_DATE" == "null" ]]; then
    echo "Could not find current version (v${CURRENT_OPENAPI_GENERATION_VERSION}) release in ${OPENAPI_GENERATION_REPOSITORY}"
    exit 1
  fi

  PRS=$(gh pr list --repo "$OPENAPI_GENERATION_REPOSITORY" --state merged --search "merged:>${START_DATE}" --json title,url | jq -r '.[] | .title+"\n > "+.url+"\n"')
#  echo "  => PRS=${PRS}"
  LATEST_OPENAPI_GENERATION_VERSION=$(gh release list --limit 1 --repo "$OPENAPI_GENERATION_REPOSITORY" --json tagName | jq -r '.[0].tagName')
#  echo "  => LATEST_OPENAPI_GENERATION_VERSION=${LATEST_OPENAPI_GENERATION_VERSION}"

  read -r SEMVER_CHANGE <<<"$("${SCRIPT_DIR}/semver.bash" diff "${CURRENT_OPENAPI_GENERATION_VERSION}" "${LATEST_OPENAPI_GENERATION_VERSION}")"
#  echo "  => SEMVER_CHANGE=${SEMVER_CHANGE}"

  if [[ "$SEMVER_CHANGE" == "none" || -z $SEMVER_CHANGE ]]; then
    echo "  => No SEMVER_CHANGE detected in downstream library. Exiting"
    exit 0
  fi

  echo "  ===== Pull Requests ==== "
  while IFS= read -r PR; do
    echo -e "${PR}"
  done <<< "$PRS"
  echo "  ===== End Pull Requests ==== "
  if command -v gum &> /dev/null; then
    SUMMARY=$(gum input --placeholder "Commit Message (please summarize changes above): ")
  else
    echo "⚠️  Install gum for a better DX: https://github.com/charmbracelet/gum"
    read -r -p "Commit Message (please summarize changes above): " SUMMARY
  fi

  go mod edit -require="${MODULE_PATH}@${LATEST_OPENAPI_GENERATION_VERSION}"
  go mod tidy

  echo "$ git add go.mod go.sum"
  git add go.mod go.sum
  echo "$ git commit --allow-empty-message -m \"$SUMMARY\""
  git commit --allow-empty-message -m "$SUMMARY"
  echo "===== When you are ready, execute the following command to upgrade ====="
  echo "$ git push origin main"
)
