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

  read -r ALL_OPENAPI_GENERATION_VERSION <<<$(go list -m -versions -json github.com/speakeasy-api/openapi-generation/v2 | jq -r '.Versions | .[]' | awk '{print substr($1,2); }' | tr '\n' ' ')
  # echo "  => ALL_OPENAPI_GENERATION_VERSION=${ALL_OPENAPI_GENERATION_VERSION}"
  LAST=
  START_DATE=
  for VERSION in $ALL_OPENAPI_GENERATION_VERSION; do
    read -r IS_BIGGER <<<$("${SCRIPT_DIR}/semver.bash" compare "${VERSION}" "${CURRENT_OPENAPI_GENERATION_VERSION}")
    if [[ $IS_BIGGER == "1" && -z $START_DATE ]]; then
      START_DATE=$(gh search commits "v$LAST" --repo speakeasy-api/openapi-generation --json "commit" | jq -r '.[] | .commit.committer.date')
      # echo "  => START_DATE=${START_DATE}"
      LAST=$VERSION
    fi
    LAST=$VERSION
  done

  if [[ -z $START_DATE ]]; then
    echo "Could not find any recent commits"
    exit 1
  fi

  ALL_COMMIT_MESSAGES=$(gh search commits " " --committer-date=">$START_DATE" --repo speakeasy-api/openapi-generation --json "commit" | jq '.[] | .commit.message')
#  echo "  => ALL_COMMIT_MESSAGES=${ALL_COMMIT_MESSAGES}"
  NEXT_OPENAPI_GENERATION_VERSION=$LAST
#  echo "  => NEXT_OPENAPI_GENERATION_VERSION=${NEXT_OPENAPI_GENERATION_VERSION}"

  read -r SEMVER_CHANGE <<<$("${SCRIPT_DIR}/semver.bash" diff "${CURRENT_OPENAPI_GENERATION_VERSION}" "${NEXT_OPENAPI_GENERATION_VERSION}")
#  echo "  => SEMVER_CHANGE=${SEMVER_CHANGE}"

  if [[ "$SEMVER_CHANGE" == "none" || -z $SEMVER_CHANGE ]]; then
    echo "  => No SEMVER_CHANGE detected in downstream library. Exiting"
    exit 0
  fi

  read -r BUMPED_CURRENT_VERSION <<<$("${SCRIPT_DIR}/semver.bash" bump "${SEMVER_CHANGE}" "${CURRENT_VERSION}")
#  echo "  => BUMPED_CURRENT_VERSION=${BUMPED_CURRENT_VERSION}"
  echo "  ===== Commit Messages ==== "
  echo "$ALL_COMMIT_MESSAGES"
  echo "  ===== End Commit Messages ==== "
  read -p "Commit Message (please summarize changes above): " SUMMARY

  go get -v "github.com/speakeasy-api/openapi-generation/v2@v${NEXT_OPENAPI_GENERATION_VERSION}"
  go mod tidy

  echo "$ git add go.mod go.sum"
  git add go.mod go.sum
  echo "$ git commit -m\"$SUMMARY\""
  git commit -m"$SUMMARY"
  echo "===== When you are ready, execute the following command to upgrade ====="
  echo "$ git push origin main"
)
