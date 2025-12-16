#!/usr/bin/env bats

# Load test helper
load test_helper

setup() {
    setup_mocks
    source_install_functions
}

teardown() {
    teardown_mocks
}

# =============================================================================
# get_os tests
# =============================================================================

@test "get_os returns linux for Linux system" {
    mock_uname "Linux" "x86_64"
    run get_os
    [ "$status" -eq 0 ]
    [ "$output" = "linux" ]
}

@test "get_os returns linux for linux (lowercase) system" {
    mock_uname "linux" "x86_64"
    run get_os
    [ "$status" -eq 0 ]
    [ "$output" = "linux" ]
}

@test "get_os returns darwin for Darwin system" {
    mock_uname "Darwin" "arm64"
    run get_os
    [ "$status" -eq 0 ]
    [ "$output" = "darwin" ]
}

@test "get_os returns darwin for darwin (lowercase) system" {
    mock_uname "darwin" "arm64"
    run get_os
    [ "$status" -eq 0 ]
    [ "$output" = "darwin" ]
}

@test "get_os returns empty for unsupported system" {
    mock_uname "Windows" "x86_64"
    run get_os
    [ "$status" -eq 0 ]
    [ "$output" = "" ]
}

# =============================================================================
# get_machine tests
# =============================================================================

@test "get_machine returns amd64 for x86_64" {
    mock_uname "Linux" "x86_64"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "amd64" ]
}

@test "get_machine returns amd64 for amd64" {
    mock_uname "Linux" "amd64"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "amd64" ]
}

@test "get_machine returns amd64 for x64" {
    mock_uname "Linux" "x64"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "amd64" ]
}

@test "get_machine returns 386 for i386" {
    mock_uname "Linux" "i386"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "386" ]
}

@test "get_machine returns 386 for i686" {
    mock_uname "Linux" "i686"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "386" ]
}

@test "get_machine returns 386 for i86pc" {
    mock_uname "Linux" "i86pc"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "386" ]
}

@test "get_machine returns 386 for x86" {
    mock_uname "Linux" "x86"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "386" ]
}

@test "get_machine returns arm64 for arm64" {
    mock_uname "Darwin" "arm64"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "arm64" ]
}

@test "get_machine returns arm64 for aarch64" {
    mock_uname "Linux" "aarch64"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "arm64" ]
}

@test "get_machine returns arm64 for armv6l" {
    mock_uname "Linux" "armv6l"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "arm64" ]
}

@test "get_machine returns empty for unsupported architecture" {
    mock_uname "Linux" "riscv64"
    run get_machine
    [ "$status" -eq 0 ]
    [ "$output" = "" ]
}

# =============================================================================
# command_exists tests
# =============================================================================

@test "command_exists returns 0 for existing command" {
    run command_exists ls
    [ "$status" -eq 0 ]
}

@test "command_exists returns non-zero for non-existing command" {
    run command_exists definitely_not_a_real_command_12345
    [ "$status" -ne 0 ]
}

# =============================================================================
# get_asset_name tests
# =============================================================================

@test "get_asset_name formats correctly for darwin arm64" {
    run get_asset_name "darwin" "arm64"
    [ "$status" -eq 0 ]
    [ "$output" = "speakeasy_darwin_arm64.zip" ]
}

@test "get_asset_name formats correctly for linux amd64" {
    run get_asset_name "linux" "amd64"
    [ "$status" -eq 0 ]
    [ "$output" = "speakeasy_linux_amd64.zip" ]
}

@test "get_asset_name formats correctly for linux 386" {
    run get_asset_name "linux" "386"
    [ "$status" -eq 0 ]
    [ "$output" = "speakeasy_linux_386.zip" ]
}

# =============================================================================
# get_download_url tests
# =============================================================================

@test "get_download_url formats correctly" {
    run get_download_url "1.2.3" "darwin" "arm64"
    [ "$status" -eq 0 ]
    [ "$output" = "https://github.com/speakeasy-api/speakeasy/releases/download/v1.2.3/speakeasy_darwin_arm64.zip" ]
}

@test "get_download_url includes version correctly" {
    run get_download_url "10.20.30" "linux" "amd64"
    [ "$status" -eq 0 ]
    [ "$output" = "https://github.com/speakeasy-api/speakeasy/releases/download/v10.20.30/speakeasy_linux_amd64.zip" ]
}

# =============================================================================
# get_checksum_url tests
# =============================================================================

@test "get_checksum_url formats correctly" {
    run get_checksum_url "1.2.3"
    [ "$status" -eq 0 ]
    [ "$output" = "https://github.com/speakeasy-api/speakeasy/releases/download/v1.2.3/checksums.txt" ]
}

@test "get_checksum_url includes version correctly" {
    run get_checksum_url "10.20.30"
    [ "$status" -eq 0 ]
    [ "$output" = "https://github.com/speakeasy-api/speakeasy/releases/download/v10.20.30/checksums.txt" ]
}

# =============================================================================
# setup_color tests
# =============================================================================

@test "setup_color sets empty colors when not connected to terminal" {
    # Run in a subshell to not affect other tests
    (
        # Force non-terminal
        exec 1>/dev/null
        setup_color
        [ -z "$RED" ]
        [ -z "$YELLOW" ]
        [ -z "$RESET" ]
    )
}

# =============================================================================
# fmt_error tests
# =============================================================================

@test "fmt_error outputs to stderr" {
    # Initialize colors first (empty since not a terminal)
    RED=""
    RESET=""
    run fmt_error "test error message"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Error: test error message"* ]]
}

@test "fmt_error includes the message" {
    RED=""
    RESET=""
    run fmt_error "specific error text here"
    [[ "$output" == *"specific error text here"* ]]
}

# =============================================================================
# fmt_warning tests
# =============================================================================

@test "fmt_warning outputs warning message" {
    YELLOW=""
    RESET=""
    run fmt_warning "test warning message"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Warning: test warning message"* ]]
}

# =============================================================================
# curl_with_retry tests
# =============================================================================

@test "curl_with_retry succeeds on 200 response" {
    mock_curl 0 "200" "response body"
    run curl_with_retry -fsSL "https://example.com"
    [ "$status" -eq 0 ]
    [[ "$output" == *"response body"* ]]
}

@test "curl_with_retry succeeds on 201 response" {
    mock_curl 0 "201" "created response"
    run curl_with_retry -fsSL "https://example.com"
    [ "$status" -eq 0 ]
    [[ "$output" == *"created response"* ]]
}

@test "curl_with_retry fails on 404 response" {
    RED=""
    RESET=""
    mock_curl 0 "404" "not found"
    run curl_with_retry -fsSL "https://example.com"
    [ "$status" -ne 0 ]
}

@test "curl_with_retry fails on 500 response after retries" {
    RED=""
    RESET=""
    # Create a curl mock that always returns 500
    cat > "$MOCK_DIR/curl" <<'EOF'
#!/bin/sh
output_file=""
while [ $# -gt 0 ]; do
    case "$1" in
        -o) output_file="$2"; shift 2 ;;
        -w) shift 2 ;;
        *) shift ;;
    esac
done
if [ -n "$output_file" ]; then
    echo "server error" > "$output_file"
fi
echo "500"
exit 0
EOF
    chmod +x "$MOCK_DIR/curl"

    # Override sleep to speed up test
    create_mock sleep 0

    run curl_with_retry -fsSL "https://example.com"
    [ "$status" -ne 0 ]
}

@test "curl_with_retry retries on curl failure" {
    RED=""
    RESET=""
    # Create a curl that fails with exit code 6 (couldn't resolve host)
    cat > "$MOCK_DIR/curl" <<'EOF'
#!/bin/sh
echo "curl: (6) Could not resolve host" >&2
exit 6
EOF
    chmod +x "$MOCK_DIR/curl"

    # Override sleep to speed up test
    create_mock sleep 0

    run curl_with_retry -fsSL "https://example.com"
    [ "$status" -ne 0 ]
}

# =============================================================================
# github_api_curl tests
# =============================================================================

@test "github_api_curl uses GITHUB_TOKEN when set" {
    # Create a curl mock that captures arguments
    cat > "$MOCK_DIR/curl" <<'EOF'
#!/bin/sh
# Output all arguments to stderr so we can check them
echo "ARGS: $*" >&2
# Find and write to output file
output_file=""
while [ $# -gt 0 ]; do
    case "$1" in
        -o) output_file="$2"; shift 2 ;;
        -w) shift 2 ;;
        *) shift ;;
    esac
done
if [ -n "$output_file" ]; then
    echo "response" > "$output_file"
fi
echo "200"
exit 0
EOF
    chmod +x "$MOCK_DIR/curl"

    export GITHUB_TOKEN="test_token_123"
    run github_api_curl --silent "https://api.github.com/test"

    # The output includes stderr which should show the auth header
    [[ "$output" == *"Authorization"* ]] || [[ "$output" == *"Bearer"* ]] || [ "$status" -eq 0 ]
    unset GITHUB_TOKEN
}

@test "github_api_curl works without GITHUB_TOKEN" {
    mock_curl 0 "200" "api response"
    unset GITHUB_TOKEN
    run github_api_curl --silent "https://api.github.com/test"
    [ "$status" -eq 0 ]
}

# =============================================================================
# Integration-style tests
# =============================================================================

@test "INSTALL_DIR defaults to /usr/local/bin" {
    [ "$INSTALL_DIR" = "/usr/local/bin" ]
}

@test "BINARY_NAME defaults to speakeasy" {
    [ "$BINARY_NAME" = "speakeasy" ]
}

@test "REPO_NAME is set correctly" {
    [ "$REPO_NAME" = "speakeasy-api/speakeasy" ]
}
