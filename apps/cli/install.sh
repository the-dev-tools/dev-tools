#!/bin/bash

set -e

# DevTools CLI Installer Script
# This script downloads and installs the DevTools CLI from GitHub releases

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
REPO_OWNER="the-dev-tools"
REPO_NAME="dev-tools"
BINARY_NAME="devtools"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Functions
print_error() {
    echo -e "${RED}Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_info() {
    echo -e "${YELLOW}$1${NC}"
}

detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case "$os" in
        linux)
            os="linux"
            ;;
        darwin)
            os="darwin"
            ;;
        msys*|mingw*|cygwin*)
            os="windows"
            ;;
        *)
            print_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    case "$arch" in
        x86_64|amd64)
            arch="x64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        i386|i686)
            if [ "$os" = "windows" ]; then
                arch="ia32"
            else
                print_error "32-bit architecture not supported on $os"
                exit 1
            fi
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    echo "${os}-${arch}"
}

get_latest_version() {
    # Fetch the package.json from main branch to get the latest version
    local package_url="https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/refs/heads/main/apps/cli/package.json"
    local version=$(curl -s "$package_url" | grep '"version"' | head -1 | sed -E 's/.*"version": "([^"]+)".*/\1/')
    
    if [ -z "$version" ]; then
        print_error "Failed to fetch latest version from package.json"
        exit 1
    fi
    
    # Verify the release exists
    local release_url="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/tags/cli@${version}"
    local release_check=$(curl -s -o /dev/null -w "%{http_code}" "$release_url")
    
    if [ "$release_check" != "200" ]; then
        print_error "Release cli@${version} not found. It may not be published yet."
        exit 1
    fi
    
    echo "$version"
}

download_binary() {
    local version=$1
    local platform=$2
    local binary_suffix=""
    
    if [[ "$platform" == "windows"* ]]; then
        binary_suffix=".exe"
    fi
    
    local binary_name="devtools-cli-${version}-${platform}${binary_suffix}"
    local download_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/cli@${version}/${binary_name}"
    local temp_file="/tmp/${binary_name}"
    
    print_info "Downloading DevTools CLI ${version} for ${platform}..."
    
    if command -v curl &> /dev/null; then
        curl -L -o "$temp_file" "$download_url" || {
            print_error "Failed to download binary"
            exit 1
        }
    elif command -v wget &> /dev/null; then
        wget -O "$temp_file" "$download_url" || {
            print_error "Failed to download binary"
            exit 1
        }
    else
        print_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    echo "$temp_file"
}

verify_checksum() {
    local binary_file=$1
    local version=$2
    local platform=$3
    local checksum_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/cli@${version}/checksums.txt"
    local temp_checksum="/tmp/devtools-checksums.txt"
    
    print_info "Verifying checksum..."
    
    if command -v curl &> /dev/null; then
        curl -sL -o "$temp_checksum" "$checksum_url" 2>/dev/null || return 0
    elif command -v wget &> /dev/null; then
        wget -q -O "$temp_checksum" "$checksum_url" 2>/dev/null || return 0
    fi
    
    if [ -f "$temp_checksum" ] && command -v sha256sum &> /dev/null; then
        local expected_checksum=$(grep "$(basename "$binary_file")" "$temp_checksum" | awk '{print $1}')
        if [ -n "$expected_checksum" ]; then
            local actual_checksum=$(sha256sum "$binary_file" | awk '{print $1}')
            if [ "$expected_checksum" != "$actual_checksum" ]; then
                print_error "Checksum verification failed"
                rm -f "$temp_checksum"
                exit 1
            fi
            print_success "Checksum verified"
        fi
        rm -f "$temp_checksum"
    fi
}

install_binary() {
    local binary_file=$1
    local install_path="${INSTALL_DIR}/${BINARY_NAME}"
    
    # Check if we need sudo
    local sudo_cmd=""
    if [ ! -w "$INSTALL_DIR" ]; then
        if command -v sudo &> /dev/null; then
            sudo_cmd="sudo"
            print_info "Administrator privileges required to install to $INSTALL_DIR"
        else
            print_error "Cannot write to $INSTALL_DIR and sudo is not available"
            exit 1
        fi
    fi
    
    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        $sudo_cmd mkdir -p "$INSTALL_DIR" || {
            print_error "Failed to create installation directory"
            exit 1
        }
    fi
    
    # Install the binary
    $sudo_cmd mv "$binary_file" "$install_path" || {
        print_error "Failed to install binary"
        exit 1
    }
    
    # Make it executable
    $sudo_cmd chmod +x "$install_path" || {
        print_error "Failed to make binary executable"
        exit 1
    }
    
    print_success "DevTools CLI installed successfully to $install_path"
}

check_prerequisites() {
    # Check for curl or wget
    if ! command -v curl &> /dev/null && ! command -v wget &> /dev/null; then
        print_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
}

main() {
    print_info "DevTools CLI Installer"
    
    check_prerequisites
    
    # Detect platform
    local platform=$(detect_platform)
    print_info "Detected platform: $platform"
    
    # Get latest version
    local version=$(get_latest_version)
    print_info "Latest version: $version"
    
    # Download binary
    local binary_file=$(download_binary "$version" "$platform")
    
    # Verify checksum if possible
    verify_checksum "$binary_file" "$version" "$platform"
    
    # Install binary
    install_binary "$binary_file"
    
    # Verify installation
    if command -v "$BINARY_NAME" &> /dev/null; then
        print_success "Installation complete! Run '${BINARY_NAME} --version' to verify."
    else
        print_info "Installation complete! You may need to add ${INSTALL_DIR} to your PATH."
        print_info "Run 'export PATH=\$PATH:${INSTALL_DIR}' to add it to your current session."
    fi
}

# Run main function
main "$@"