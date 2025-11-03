#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPO_ROOT="${SCRIPT_DIR}/../.."

cd "${REPO_ROOT}"

# Set up environment
export GOPRIVATE=github.com/speakeasy-api/*
export GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN}}"

# Ensure we're on main branch
git checkout main || git checkout -b main
git pull origin main || true

# Get current version
CURRENT_VERSION=$(git describe --tags 2>/dev/null | awk '{print substr($1,2); }' || echo "0.0.0")
echo "Current version: ${CURRENT_VERSION}"

# Get current openapi-generation version from go.mod
CURRENT_OPENAPI_GENERATION_VERSION=$(grep "github.com/speakeasy-api/openapi-generation/v2" go.mod | awk '{ print $2 }' | sed 's/v//')
echo "Current openapi-generation version: ${CURRENT_OPENAPI_GENERATION_VERSION}"

# Get start date from the release
START_DATE=$(gh release view "v${CURRENT_OPENAPI_GENERATION_VERSION}" --repo speakeasy-api/openapi-generation --json createdAt -q '.createdAt' 2>/dev/null || echo "")

if [[ -z "$START_DATE" ]]; then
  echo "Could not find release v${CURRENT_OPENAPI_GENERATION_VERSION}, exiting"
  exit 0
fi

# Get latest openapi-generation version
LATEST_OPENAPI_GENERATION_VERSION=$(gh release list --limit 1 --repo speakeasy-api/openapi-generation --json tagName -q '.[0].tagName' | sed 's/v//')
echo "Latest openapi-generation version: ${LATEST_OPENAPI_GENERATION_VERSION}"

# Check if there's a version difference using semver.bash
SEMVER_CHANGE=$("${REPO_ROOT}/scripts/semver.bash" diff "${CURRENT_OPENAPI_GENERATION_VERSION}" "${LATEST_OPENAPI_GENERATION_VERSION}" || echo "none")

if [[ "$SEMVER_CHANGE" == "none" || -z "$SEMVER_CHANGE" ]]; then
  echo "No semver change detected, exiting"
  exit 0
fi

echo "Semver change detected: ${SEMVER_CHANGE}"

# Get merged PRs since START_DATE
echo "Fetching merged PRs since ${START_DATE}..."
PRS_JSON=$(gh pr list --repo speakeasy-api/openapi-generation --state merged --search "merged:>${START_DATE}" --json number,title,url,body,labels,files --limit 100)

# Check if we have any PRs
PR_COUNT=$(echo "$PRS_JSON" | jq 'length')
if [[ "$PR_COUNT" -eq 0 ]]; then
  echo "No merged PRs found, exiting"
  exit 0
fi

echo "Found ${PR_COUNT} merged PRs"

# Filter out internal PRs and group by language
declare -A lang_prs
declare -A lang_summaries
core_prs=()

# Language mapping (normalize v2 suffixes)
normalize_lang() {
  local lang="$1"
  lang=$(echo "$lang" | sed 's/v2$//' | tr '[:upper:]' '[:lower:]')
  echo "$lang"
}

# Check if PR is internal
is_internal_pr() {
  local pr_num="$1"
  local title="$2"
  
  # Filter out chore: PRs
  if [[ "$title" =~ ^[cC]hore: ]]; then
    return 0
  fi
  
  # Check if PR has changelog files
  local files=$(gh pr view "$pr_num" --repo speakeasy-api/openapi-generation --json files -q '.[].path' 2>/dev/null || echo "")
  if echo "$files" | grep -q "changelogs/"; then
    return 1  # Has changelogs, not internal
  fi
  
  return 0  # No changelogs, internal
}

# Process each PR
while IFS= read -r pr_data; do
  pr_num=$(echo "$pr_data" | jq -r '.number')
  title=$(echo "$pr_data" | jq -r '.title')
  url=$(echo "$pr_data" | jq -r '.url')
  body=$(echo "$pr_data" | jq -r '.body // ""')
  
  # Check if internal
  if is_internal_pr "$pr_num" "$title"; then
    echo "Skipping internal PR: #${pr_num} - ${title}"
    continue
  fi
  
  # Extract language from labels or title
  lang=""
  labels=$(echo "$pr_data" | jq -r '.labels[].name' | grep -E "(python|typescript|java|go|csharp|php|ruby|terraform)" || true)
  
  if [[ -n "$labels" ]]; then
    lang=$(echo "$labels" | head -1)
  else
    # Try to extract from title
    if echo "$title" | grep -qiE "(python|typescript|java|go|csharp|php|ruby|terraform)"; then
      lang=$(echo "$title" | grep -oiE "(pythonv2|typescriptv2|python|typescript|java|go|csharp|php|ruby|terraform)" | head -1)
    fi
  fi
  
  lang=$(normalize_lang "$lang")
  
  # Check files for language hints
  if [[ -z "$lang" ]]; then
    files=$(echo "$pr_data" | jq -r '.files[].path' 2>/dev/null || echo "")
    for file in $files; do
      if echo "$file" | grep -qE "(python|typescript|java|go|csharp|php|ruby|terraform)"; then
        lang=$(echo "$file" | grep -oiE "(pythonv2|typescriptv2|python|typescript|java|go|csharp|php|ruby|terraform)" | head -1)
        lang=$(normalize_lang "$lang")
        break
      fi
    done
  fi
  
  # Create user-facing summary (simplified - extract from body or title)
  summary="$title"
  if [[ -n "$body" && "$body" != "null" ]]; then
    # Try to extract a summary from the body (first sentence or first line)
    first_line=$(echo "$body" | head -1 | sed 's/^#* *//' | sed 's/\*\*//g')
    if [[ ${#first_line} -lt 200 ]]; then
      summary="$first_line"
    fi
  fi
  
  if [[ -n "$lang" && "$lang" != "" ]]; then
    if [[ -z "${lang_prs[$lang]:-}" ]]; then
      lang_prs["$lang"]=""
      lang_summaries["$lang"]=""
    fi
    lang_prs["$lang"]="${lang_prs[$lang]}- [${summary}](${url})"$'\n'
  else
    # Core or unknown language
    core_prs+=("- [${summary}](${url})")
  fi
  
done < <(echo "$PRS_JSON" | jq -c '.[]')

# Calculate new version
BUMPED_VERSION=$("${REPO_ROOT}/scripts/semver.bash" bump "${SEMVER_CHANGE}" "${CURRENT_VERSION}")
echo "Bumped version: ${BUMPED_VERSION}"

# Check if a release PR already exists
BRANCH_NAME="release/v${BUMPED_VERSION}"
EXISTING_PR=$(gh pr list --head "$BRANCH_NAME" --state open --json number -q '.[0].number' 2>/dev/null || echo "")

if [[ -n "$EXISTING_PR" ]]; then
  echo "Release PR #${EXISTING_PR} already exists for ${BRANCH_NAME}, exiting"
  exit 0
fi

# Create branch
git checkout -b "$BRANCH_NAME" 2>/dev/null || git checkout "$BRANCH_NAME"

# Update go.mod
go get -v "github.com/speakeasy-api/openapi-generation/v2@v${LATEST_OPENAPI_GENERATION_VERSION}"
go mod tidy

# Check if there are changes
if git diff --quiet go.mod go.sum; then
  echo "No changes to go.mod/go.sum, exiting"
  exit 0
fi

# Build PR title
pr_title_parts=()
for lang in "${!lang_prs[@]}"; do
  # Determine if features or fixes (simplified - check for "fix" in summaries)
  fixes=""
  features=""
  if echo "${lang_prs[$lang]}" | grep -qi "fix"; then
    fixes="fix"
  fi
  if echo "${lang_prs[$lang]}" | grep -qiE "(feat|add|new|support)"; then
    features="feat"
  fi
  
  if [[ -n "$features" && -n "$fixes" ]]; then
    pr_title_parts+=("feat(${lang}): updates; fix(${lang}): fixes")
  elif [[ -n "$features" ]]; then
    pr_title_parts+=("feat(${lang}): updates")
  elif [[ -n "$fixes" ]]; then
    pr_title_parts+=("fix(${lang}): fixes")
  else
    pr_title_parts+=("chore(${lang}): updates")
  fi
done

if [[ ${#pr_title_parts[@]} -eq 0 && ${#core_prs[@]} -gt 0 ]]; then
  pr_title="chore: update dependencies"
elif [[ ${#pr_title_parts[@]} -gt 0 ]]; then
  pr_title=$(IFS="; "; echo "${pr_title_parts[*]}")
else
  pr_title="chore: update dependencies"
fi

# Build PR description
pr_body="## Core"$'\n'
if [[ ${#core_prs[@]} -gt 0 ]]; then
  for pr in "${core_prs[@]}"; do
    pr_body+="${pr}"$'\n'
  done
else
  pr_body+="- Dependency updates"$'\n'
fi
pr_body+=$'\n'

# Add language sections
for lang in python typescript java go csharp php ruby terraform; do
  if [[ -n "${lang_prs[$lang]:-}" ]]; then
    pr_body+="## ${lang^}"$'\n'
    pr_body+="${lang_prs[$lang]}"
    pr_body+=$'\n'
  fi
done

# Commit changes
git add go.mod go.sum
git -c user.name="speakeasybot" -c user.email="bot@speakeasyapi.dev" commit -m "chore: bump openapi-generation to v${LATEST_OPENAPI_GENERATION_VERSION}" || true

# Push branch
git push origin "$BRANCH_NAME" || git push -f origin "$BRANCH_NAME"

# Create PR
echo "Creating PR..."
gh pr create \
  --title "$pr_title" \
  --body "$pr_body" \
  --head "$BRANCH_NAME" \
  --base main \
  --repo "$(gh repo view --json nameWithOwner -q .nameWithOwner)"

echo "Release PR created successfully!"

