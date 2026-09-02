#!/usr/bin/env bash
set -euo pipefail

# compile-release-notes.sh
#
# Compiles release notes from openapi-generation releases that occurred between
# the previous and current CLI release, then appends them to the CLI's GitHub
# release notes.
#
# Usage: compile-release-notes.sh <current-tag> [previous-tag]
#
# Requires: gh (GitHub CLI), jq, git

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
REPO_DIR="${SCRIPT_DIR}/.."
OG_REPO="speakeasy-api/openapi-generation"
CLI_REPO="speakeasy-api/speakeasy"

CURRENT_TAG="${1:?Usage: compile-release-notes.sh <current-tag> [previous-tag]}"
PREV_TAG="${2:-}"

# Find the previous non-RC release tag if not provided
if [[ -z "$PREV_TAG" ]]; then
  PREV_TAG=$(gh release list --repo "$CLI_REPO" --limit 50 --json tagName \
    | jq -r '[.[] | select(.tagName | test("-rc") | not)] | .[1].tagName')
fi

if [[ -z "$PREV_TAG" ]]; then
  echo "Could not determine previous release tag"
  exit 0
fi

echo "CLI release range: $PREV_TAG -> $CURRENT_TAG"

# Extract the openapi-generation version from go.mod at a given git ref
get_og_version() {
  local ref="$1"
  git -C "$REPO_DIR" show "${ref}:go.mod" \
    | grep 'github.com/speakeasy-api/openapi-generation/v2 ' \
    | awk '{print $2}'
}

CURRENT_OG_VERSION=$(get_og_version "$CURRENT_TAG")
PREV_OG_VERSION=$(get_og_version "$PREV_TAG")

if [[ -z "$CURRENT_OG_VERSION" || -z "$PREV_OG_VERSION" ]]; then
  echo "Could not extract openapi-generation versions from go.mod"
  exit 0
fi

echo "openapi-generation range: $PREV_OG_VERSION -> $CURRENT_OG_VERSION"

if [[ "$CURRENT_OG_VERSION" == "$PREV_OG_VERSION" ]]; then
  echo "No openapi-generation version change between releases"
  exit 0
fi

# Check if the release has already been updated (idempotency)
# If the body already contains target/language headers, it's been compiled
CURRENT_BODY=$(gh release view "$CURRENT_TAG" --repo "$CLI_REPO" --json body -q '.body')
if echo "$CURRENT_BODY" | grep -q "^### All Targets\|^### TypeScript\|^### Python\|^### Go\|^### Terraform"; then
  echo "Release notes already compiled, skipping"
  exit 0
fi

# Step 1: Get recent release tags from openapi-generation (fast, no body)
# 200 is more than enough to cover the gap between two CLI releases
TAGS_IN_RANGE=$(gh release list -R "$OG_REPO" --limit 200 --json tagName \
  | jq -r --arg prev "$PREV_OG_VERSION" --arg curr "$CURRENT_OG_VERSION" '
  def parse_ver: ltrimstr("v") | split(".") | map(tonumber);
  def ver_cmp(a; b):
    if (a | length) == 0 and (b | length) == 0 then 0
    elif (a | length) == 0 then -1
    elif (b | length) == 0 then 1
    elif a[0] < b[0] then -1
    elif a[0] > b[0] then 1
    else ver_cmp(a[1:]; b[1:])
    end;
  ($prev | parse_ver) as $pv |
  ($curr | parse_ver) as $cv |
  [ .[]
    | select(.tagName | test("^v[0-9]+\\.[0-9]+\\.[0-9]+$"))
    | (.tagName | parse_ver) as $tv
    | select(ver_cmp($tv; $pv) > 0 and ver_cmp($tv; $cv) <= 0)
  ]
  | sort_by(.tagName | parse_ver)
  | reverse
  | .[].tagName
')

if [[ -z "$TAGS_IN_RANGE" ]]; then
  echo "No openapi-generation releases found in range ($PREV_OG_VERSION, $CURRENT_OG_VERSION]"
  exit 0
fi

echo "Found releases in range:"
echo "$TAGS_IN_RANGE"

# Step 2: Fetch all release bodies and concatenate
ALL_BODIES=""
while IFS= read -r tag; do
  echo "  Fetching notes for $tag..."
  BODY=$(gh release view "$tag" -R "$OG_REPO" --json body -q '.body')
  if [[ -n "$BODY" ]]; then
    ALL_BODIES+="${BODY}"$'\n'
  fi
done <<< "$TAGS_IN_RANGE"

if [[ -z "$ALL_BODIES" ]]; then
  echo "All releases in range had empty bodies"
  exit 0
fi

# Step 3: Group items by target/language, then by change type within each
COMPILED_NOTES=$(echo "$ALL_BODIES" | awk '
BEGIN {
  # Display names for targets
  dn["all"] = "All Targets"
  dn["csharp"] = "C#"
  dn["go"] = "Go"
  dn["java"] = "Java"
  dn["php"] = "PHP"
  dn["python"] = "Python"
  dn["ruby"] = "Ruby"
  dn["swift"] = "Swift"
  dn["terraform"] = "Terraform"
  dn["typescript"] = "TypeScript"
  dn["unity"] = "Unity"

  cats[1] = "New Features"
  cats[2] = "Bug Fixes"
  cats[3] = "Chores"
  nCats = 3
}

/^### :/ {
  c = $0
  if (c ~ /New Features/) cat = "New Features"
  else if (c ~ /Bug Fixes/) cat = "Bug Fixes"
  else if (c ~ /Chores/) cat = "Chores"
  else { sub(/^### :[^:]+: /, "", c); cat = c }
  next
}

/^- / {
  if (cat == "") next
  line = $0
  scope = ""

  # Extract **scope**: if present
  if (match(line, /\*\*[^*]+\*\*/)) {
    scope = substr(line, RSTART + 2, RLENGTH - 4)
    # Take first part of comma-separated scopes
    if (index(scope, ",") > 0) {
      scope = substr(scope, 1, index(scope, ",") - 1)
    }
    # Remove **scope**: from the line (redundant once grouped)
    sub(/\*\*[^*]+\*\*: /, "", line)
  }

  if (scope == "") {
    unscoped = unscoped line "\n"
  } else {
    key = scope "|" cat
    items[key] = items[key] line "\n"
    if (!(scope in scopeSeen)) {
      scopeSeen[scope] = 1
      scopeList[++nScopes] = scope
    }
  }
  next
}

END {
  # Fixed target order
  nOrder = split("all,typescript,python,go,terraform,java,ruby,csharp,php", order, ",")

  # Print targets in the defined order
  for (o = 1; o <= nOrder; o++) {
    sc = order[o]
    name = (sc in dn) ? dn[sc] : sc
    header_printed = 0
    for (c = 1; c <= nCats; c++) {
      key = sc "|" cats[c]
      if (key in items) {
        if (!header_printed) {
          print "### " name
          header_printed = 1
        }
        print "**" cats[c] "**"
        printf "%s", items[key]
      }
    }
    if (header_printed) print ""
    delete scopeSeen[sc]
  }

  # Print any remaining targets not in the predefined order
  for (s = 1; s <= nScopes; s++) {
    sc = scopeList[s]
    if (!(sc in scopeSeen)) continue
    name = (sc in dn) ? dn[sc] : sc
    header_printed = 0
    for (c = 1; c <= nCats; c++) {
      key = sc "|" cats[c]
      if (key in items) {
        if (!header_printed) {
          print "### " name
          header_printed = 1
        }
        print "**" cats[c] "**"
        printf "%s", items[key]
      }
    }
    if (header_printed) print ""
  }

  if (unscoped != "") {
    print "### Chores"
    printf "%s", unscoped
    print ""
  }
}
')

# Update the GitHub release, replacing the body entirely
echo "$COMPILED_NOTES" | gh release edit "$CURRENT_TAG" --repo "$CLI_REPO" --notes-file -

echo "Successfully updated release notes for $CURRENT_TAG with openapi-generation changes"
