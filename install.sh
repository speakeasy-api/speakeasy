#!/bin/sh
set -e

# Adapted/Copied from https://raw.githubusercontent.com/daveshanley/vacuum/main/bin/install.sh
#
# Speakeasy CLI
# https://speakeasyapi.dev/docs/speakeasy-cli/getting-started/
#
# Designed for quick installs over the network and CI/CD
#   curl -fsSL https://raw.githubusercontent.com/speakeasy-api/speakeasy/main/install.sh | sh

INSTALL_DIR=${INSTALL_DIR:-"/usr/local/bin"}
BINARY_NAME=${BINARY_NAME:-"speakeasy"}

REPO_NAME="speakeasy-api/speakeasy"
ISSUE_URL="https://github.com/speakeasy-api/speakeasy/issues/new"

# get_latest_release "speakeasy-api/speakeasy"
get_latest_release() {
  local retry=0
  local max_retries=5
  local release_info
  local delay=1

  while [ $retry -lt $max_retries ]; do
    release_info=$(curl --retry 5 --silent "https://api.github.com/repos/$1/releases/latest")
    tag_name=$(echo "$release_info" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if echo "$tag_name" | grep -q '^v'; then
      echo "$tag_name"
      return
    else
      sleep $delay
      delay=$((delay * 2))  # Double the delay for each retry
      retry=$((retry + 1))
    fi
  done
  echo "Error: Unable to retrieve a valid release tag after $max_retries attempts." >&2
  exit 1
}

get_asset_name() {
  echo "speakeasy_$1_$2.zip"
}

get_download_url() {
  local asset_name=$(get_asset_name $2 $3)
  echo "https://github.com/speakeasy-api/speakeasy/releases/download/v$1/${asset_name}"
}

get_checksum_url() {
  echo "https://github.com/speakeasy-api/speakeasy/releases/download/v$1/checksums.txt"
}

command_exists() {
  command -v "$@" >/dev/null 2>&1
}

fmt_error() {
  echo ${RED}"Error: $@"${RESET} >&2
}

fmt_warning() {
  echo ${YELLOW}"Warning: $@"${RESET} >&2
}

fmt_underline() {
  echo "$(printf '\033[4m')$@$(printf '\033[24m')"
}

fmt_code() {
  echo "\`$(printf '\033[38;5;247m')$@${RESET}\`"
}

setup_color() {
  # Only use colors if connected to a terminal
  if [ -t 1 ]; then
    RED=$(printf '\033[31m')
    GREEN=$(printf '\033[32m')
    YELLOW=$(printf '\033[33m')
    BLUE=$(printf '\033[34m')
    MAGENTA=$(printf '\033[35m')
    BOLD=$(printf '\033[1m')
    RESET=$(printf '\033[m')
  else
    RED=""
    GREEN=""
    YELLOW=""
    BLUE=""
    MAGENTA=""
    BOLD=""
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

get_tmp_dir() {
  echo $(mktemp -d)
}

do_checksum() {
  checksum_url=$(get_checksum_url $version)
  get_checksum_url $version
  expected_checksum=$(curl -sL $checksum_url | grep $asset_name | awk '{print $1}')



  if command_exists sha256sum; then
    checksum=$(sha256sum $asset_name | awk '{print $1}')
  elif command_exists shasum; then
    checksum=$(shasum -a 256 $asset_name | awk '{print $1}')
  else
    fmt_warning "Could not find a checksum program. Install shasum or sha256sum to validate checksum."
    return 0
  fi

  if [ "$checksum" != "$expected_checksum" ]; then
    fmt_error "Checksums do not match"
    exit 1
  fi
}

do_install_binary() {
  asset_name=$(get_asset_name $os $machine)
  download_url=$(get_download_url $version $os $machine)

  command_exists curl || {
    fmt_error "curl is not installed"
    exit 1
  }

  command_exists unzip || {
    fmt_error "unzip is not installed"
    exit 1
  }

  local tmp_dir=$(get_tmp_dir)

  # Download tar.gz to tmp directory
  echo "Downloading $download_url"
  (cd $tmp_dir && curl -sL -O "$download_url")

  (cd $tmp_dir && do_checksum)

  # Extract download
  (cd $tmp_dir && unzip -q "$asset_name")

  # Install binary
  sudo_cmd='mv '"$tmp_dir/$BINARY_NAME"' '"$INSTALL_DIR"' && chmod a+x '"$INSTALL_DIR/$BINARY_NAME"
  sudo -p "sudo password required for installing to $INSTALL_DIR: " -- sh -c "$sudo_cmd"
  echo "Installed speakeasy to $INSTALL_DIR"

  # Cleanup
  rm -rf $tmp_dir
}

main() {
  setup_color

  latest_tag=$(get_latest_release $REPO_NAME)
  latest_version=$(echo $latest_tag | sed 's/v//')
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

  printf "$YELLOW"
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
  printf "$RESET"

}

main

