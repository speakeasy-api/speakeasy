#!/bin/sh
set -e

# Adapted/Copied from https://raw.githubusercontent.com/daveshanley/vacuum/main/bin/install.sh
#
# Speakeasy CLI
# https://speakeasy.com/docs/speakeasy-cli/getting-started/
#
# Designed for quick installs over the network and CI/CD
#   curl -fsSL https://raw.githubusercontent.com/speakeasy-api/speakeasy/main/install.sh | sh

INSTALL_DIR=${INSTALL_DIR:-"/usr/local/bin"}
BINARY_NAME=${BINARY_NAME:-"speakeasy"}

REPO_NAME="speakeasy-api/speakeasy"
ISSUE_URL="https://github.com/speakeasy-api/speakeasy/issues/new"

# curl_with_retry - curl wrapper that retries on 5XX errors and shows full errors
# Usage: curl_with_retry [curl_args...]
curl_with_retry() {
  _curl_delay=1
  _curl_exit_code=0
  _curl_http_code=""
  _curl_max_retries=5
  _curl_retry=0
  _curl_tmp_file="$(mktemp)"

  while [ $_curl_retry -lt $_curl_max_retries ]; do
    # Use -w to get HTTP status code, -o to capture body, -S to show errors
    _curl_http_code=$(curl -S -w "%{http_code}" -o "$_curl_tmp_file" "$@" 2>&1)
    _curl_exit_code=$?

    # If curl itself failed (not HTTP error), show the error and retry
    if [ $_curl_exit_code -ne 0 ]; then
      # http_code contains the error message when curl fails
      fmt_error "curl failed: $_curl_http_code"
      if [ $_curl_retry -lt $((_curl_max_retries - 1)) ]; then
        echo "Retrying in ${_curl_delay}s... (attempt $((_curl_retry + 2))/${_curl_max_retries})" >&2
        sleep $_curl_delay
        _curl_delay=$((_curl_delay * 2))
        _curl_retry=$((_curl_retry + 1))
        continue
      else
        rm -f "$_curl_tmp_file"
        return $_curl_exit_code
      fi
    fi

    # Check for 5XX status codes
    case "$_curl_http_code" in
      5*)
        fmt_error "Server error (HTTP $_curl_http_code)"
        if [ $_curl_retry -lt $((_curl_max_retries - 1)) ]; then
          echo "Retrying in ${_curl_delay}s... (attempt $((_curl_retry + 2))/${_curl_max_retries})" >&2
          sleep $_curl_delay
          _curl_delay=$((_curl_delay * 2))
          _curl_retry=$((_curl_retry + 1))
          continue
        else
          rm -f "$_curl_tmp_file"
          return 1
        fi
        ;;
      4*)
        # Client error - don't retry, but show the error
        fmt_error "HTTP error $_curl_http_code"
        cat "$_curl_tmp_file" >&2
        rm -f "$_curl_tmp_file"
        return 1
        ;;
      *)
        # Success (2XX, 3XX) - output the content
        cat "$_curl_tmp_file"
        rm -f "$_curl_tmp_file"
        return 0
        ;;
    esac
  done

  rm -f "$_curl_tmp_file"
  return 1
}

# github_api_curl - make authenticated GitHub API calls
github_api_curl() {
  if [ -n "$GITHUB_TOKEN" ]; then
    curl_with_retry -H "Authorization: Bearer $GITHUB_TOKEN" "$@"
  else
    curl_with_retry "$@"
  fi
}

# get_latest_release "speakeasy-api/speakeasy"
get_latest_release() {
  _get_latest_release_repo=$1
  _get_latest_release_info=$(github_api_curl --silent "https://api.github.com/repos/${_get_latest_release_repo}/releases/latest")
  _get_latest_release_tag_name=$(echo "$_get_latest_release_info" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

  if echo "$_get_latest_release_tag_name" | grep -q '^v'; then
    echo "$_get_latest_release_tag_name"
    return
  fi

  fmt_error "Could not fetch latest release tag from GitHub API"
  exit 1
}

get_asset_name() {
  _get_asset_name_version="$1"
  _get_asset_name_platform="$2"
  echo "speakeasy_${_get_asset_name_version}_${_get_asset_name_platform}.zip"
}

get_download_url() {
  _get_download_url_version="$1"
  _get_download_url_asset_name=$(get_asset_name "$2" "$3")
  echo "https://github.com/speakeasy-api/speakeasy/releases/download/v${_get_download_url_version}/${_get_download_url_asset_name}"
}

get_checksum_url() {
  _get_checksum_url_version="$1"
  echo "https://github.com/speakeasy-api/speakeasy/releases/download/v${_get_checksum_url_version}/checksums.txt"
}

command_exists() {
  command -v "$@" >/dev/null 2>&1
}

fmt_error() {
  printf "%sError: %s%s\n" "${RED}" "$@" "${RESET}" >&2
}

fmt_warning() {
  printf "%sWarning: %s%s\n" "${YELLOW}" "$@" "${RESET}" >&2
}

setup_color() {
  # Only use colors if connected to a terminal
  if [ -t 1 ]; then
    RED=$(printf '\033[31m')
    YELLOW=$(printf '\033[33m')
    RESET=$(printf '\033[m')
  else
    RED=""
    YELLOW=""
    RESET=""
  fi
}

get_os() {
  case "$(uname -s)" in
    *linux* ) echo "linux" ;;
    *Linux* ) echo "linux" ;;
    *darwin* ) echo "darwin" ;;
    *Darwin* ) echo "darwin" ;;
  esac
}

get_machine() {
  case "$(uname -m)" in
    "x86_64"|"amd64"|"x64")
      echo "amd64" ;;
    "i386"|"i86pc"|"x86"|"i686")
      echo "386" ;;
    "arm64"|"armv6l"|"aarch64")
      echo "arm64"
  esac
}

do_checksum() {
  checksum_url=$(get_checksum_url "$version")
  checksum_content=$(curl_with_retry -fL "$checksum_url")

  if [ -z "$checksum_content" ]; then
    fmt_error "Failed to retrieve checksums from $checksum_url"
    exit 1
  fi

  expected_checksum=$(echo "$checksum_content" | grep "$asset_name" | awk '{print $1}')

  if [ -z "$expected_checksum" ]; then
    fmt_error "Failed to find checksum for $asset_name in $checksum_url"
    exit 1
  fi

  if command_exists sha256sum; then
    checksum=$(sha256sum "$asset_name" | awk '{print $1}')
  elif command_exists shasum; then
    checksum=$(shasum -a 256 "$asset_name" | awk '{print $1}')
  else
    fmt_warning "Could not find a checksum program. Install shasum or sha256sum to validate checksum."
    return 0
  fi

  if [ "$checksum" != "$expected_checksum" ]; then
    fmt_error "Checksums do not match (expected: $expected_checksum, got: $checksum)"
    exit 1
  fi
}

do_install_binary() {
  asset_name=$(get_asset_name "$os" "$machine")
  download_url=$(get_download_url "$version" "$os" "$machine")

  command_exists curl || {
    fmt_error "curl is not installed"
    exit 1
  }

  command_exists unzip || {
    fmt_error "unzip is not installed"
    exit 1
  }

  tmp_dir="$(mktemp -d)"

  # Download zip to tmp directory
  echo "Downloading $download_url"
  set +e
  (cd "$tmp_dir" && curl_with_retry -fL "$download_url" > "$asset_name")
  exit_code=$?
  set -e
  if [ $exit_code -ne 0 ]; then
    fmt_error "Failed to download $download_url"
    rm -rf "$tmp_dir"
    exit 1
  fi

  (cd "$tmp_dir" && do_checksum)

  # Extract download
  (cd "$tmp_dir" && unzip -q "$asset_name")

  # Install binary
  sudo_cmd='mv '"$tmp_dir/$BINARY_NAME"' '"$INSTALL_DIR"' && chmod a+x '"$INSTALL_DIR/$BINARY_NAME"
  sudo -p "sudo password required for installing to $INSTALL_DIR: " -- sh -c "$sudo_cmd"
  echo "Installed speakeasy to $INSTALL_DIR"

  # Cleanup
  rm -rf "$tmp_dir"
}

main() {
  setup_color

  latest_tag=$(get_latest_release "$REPO_NAME")
  latest_version=$(echo "$latest_tag" | sed 's/v//')
  version=${VERSION:-$latest_version}

  os=$(get_os)
  if test -z "$os"; then
    fmt_error "$(uname -s) os type is not supported"
    echo "Please create an issue so we can add support. $ISSUE_URL"
    exit 1
  fi

  machine=$(get_machine)
  if test -z "$machine"; then
    fmt_error "$(uname -m) machine type is not supported"
    echo "Please create an issue so we can add support. $ISSUE_URL"
    exit 1
  fi
  do_install_binary

  printf "%s" "${YELLOW}"
  cat <<'EOF'
      .-.         .--''-.
    .'   '.     /'       `.
    '.     '. ,'          |                       Buzz!
 o    '.o   ,'        _.-'
  \.--./'. /.:. :._:.'             The Speakeasy CLI is now Installed!
 .'    '._-': ': ': ': ': 
:(#) (#) :  ': ': ': ': ':>-   Run `speakeasy help` for a list of commands.
 ' ____ .'_.:' :' :' :' :'      Or just `speakeasy` for interactive mode.
  '\__/'/ | | :' :' :'
        \  \ \
        '  ' 'MJP
EOF
  printf "%s" "${RESET}"

}

main