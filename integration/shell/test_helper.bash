#!/usr/bin/env bash
# Test helper for install.sh BATS tests

# Get the directory of this script
BATS_TEST_DIRNAME="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
INSTALL_SCRIPT="$PROJECT_ROOT/install.sh"

# Source functions from install.sh without executing main
# This extracts functions by removing the final 'main' call
source_install_functions() {
    # Create temp file with functions only (remove 'set -e' and final 'main' call)
    local temp_script
    temp_script=$(mktemp)

    # Copy install.sh but comment out 'set -e' and the final 'main' call
    sed -e 's/^set -e$/# set -e (disabled for testing)/' \
        -e 's/^main$/# main (disabled for testing)/' \
        "$INSTALL_SCRIPT" > "$temp_script"

    # shellcheck source=/dev/null
    source "$temp_script"
    rm -f "$temp_script"
}

# Setup mock directory for fake commands
setup_mocks() {
    MOCK_DIR="$(mktemp -d)"
    export PATH="$MOCK_DIR:$PATH"
    export MOCK_DIR
}

# Cleanup mock directory
teardown_mocks() {
    if [[ -n "${MOCK_DIR:-}" && -d "$MOCK_DIR" ]]; then
        rm -rf "$MOCK_DIR"
    fi
}

# Create a mock command
# Usage: create_mock command_name exit_code [output]
create_mock() {
    local cmd_name="$1"
    local exit_code="${2:-0}"
    local output="${3:-}"

    cat > "$MOCK_DIR/$cmd_name" <<EOF
#!/bin/sh
${output:+echo "$output"}
exit $exit_code
EOF
    chmod +x "$MOCK_DIR/$cmd_name"
}

# Create a mock uname that returns specific values
# Usage: mock_uname "Darwin" "arm64"
mock_uname() {
    local system="$1"
    local machine="$2"

    cat > "$MOCK_DIR/uname" <<EOF
#!/bin/sh
case "\$1" in
    -s) echo "$system" ;;
    -m) echo "$machine" ;;
    *)  echo "$system" ;;
esac
EOF
    chmod +x "$MOCK_DIR/uname"
}

# Create a mock curl that returns specific content
# Usage: mock_curl exit_code http_code [body]
mock_curl() {
    local exit_code="$1"
    local http_code="$2"
    local body="${3:-}"

    cat > "$MOCK_DIR/curl" <<'OUTER_EOF'
#!/bin/sh
# Parse arguments to find -o and -w flags
output_file=""
write_out=""
while [ $# -gt 0 ]; do
    case "$1" in
        -o) output_file="$2"; shift 2 ;;
        -w) write_out="$2"; shift 2 ;;
        *) shift ;;
    esac
done
OUTER_EOF

    cat >> "$MOCK_DIR/curl" <<EOF
# Write body to output file if specified
if [ -n "\$output_file" ]; then
    echo '$body' > "\$output_file"
fi

# Handle write-out format
if [ "\$write_out" = "%{http_code}" ]; then
    echo "$http_code"
fi

exit $exit_code
EOF
    chmod +x "$MOCK_DIR/curl"
}
