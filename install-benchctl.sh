#!/bin/bash

# benchctl install script
# Supports Linux and macOS. fk windows!

set -e

REPO="luccadibe/benchctl"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

# Print output
print_info() {
    echo "[INFO] $1"
}

print_success() {
    echo "[SUCCESS] $1"
}

print_warning() {
    echo "[WARNING] $1"
}

print_error() {
    echo "[ERROR] $1"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Detect OS and architecture
detect_platform() {
    local os arch
    
    case "$(uname -s)" in
        Linux*)
            os="Linux"
            ;;
        Darwin*)
            os="Darwin"
            ;;
        *)
            print_error "Unsupported operating system: $(uname -s)"
            print_error "This script only supports Linux and macOS"
            exit 1
            ;;
    esac
    
    case "$(uname -m)" in
        x86_64|amd64)
            arch="x86_64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        i386|i686)
            if [ "$os" = "Linux" ]; then
                arch="i386"
            else
                print_error "32-bit architecture not supported on macOS"
                exit 1
            fi
            ;;
        *)
            print_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
    
    echo "${os}_${arch}"
}

# Get latest version from GitHub API
get_latest_version() {
    if [ "$VERSION" = "latest" ]; then
        print_info "Fetching latest version..."
        if command_exists curl; then
            VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
        elif command_exists wget; then
            VERSION=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
        else
            print_error "Neither curl nor wget found. Please install one of them or set VERSION manually."
            exit 1
        fi
        
        if [ -z "$VERSION" ]; then
            print_error "Failed to fetch latest version"
            exit 1
        fi
        
        print_info "Latest version: $VERSION"
    fi
}

# Download and verify checksum
download_and_verify() {
    local platform="$1"
    local version="$2"
    local filename="benchctl_${platform}.tar.gz"
    local url="https://github.com/$REPO/releases/download/${version}/${filename}"
    local temp_dir=$(mktemp -d)
    local download_path="$temp_dir/$filename"
    local checksum_url="https://github.com/$REPO/releases/download/${version}/benchctl_${version#v}_checksums.txt"
    
    print_info "Downloading benchctl ${version} for ${platform}..." >&2
    print_info "URL: $url" >&2
    
    # Download the binary
    if command_exists curl; then
        curl -L -o "$download_path" "$url" >/dev/null 2>&1
    elif command_exists wget; then
        wget -O "$download_path" "$url" >/dev/null 2>&1
    else
        print_error "Neither curl nor wget found. Please install one of them." >&2
        exit 1
    fi
    
    if [ ! -f "$download_path" ] || [ ! -s "$download_path" ]; then
        print_error "Failed to download $filename" >&2
        exit 1
    fi
    
    print_info "Downloaded successfully" >&2
    
    # Verify checksum
    print_info "Verifying checksum..." >&2
    local checksum_file="$temp_dir/checksums.txt"
    
    if command_exists curl; then
        curl -s -L -o "$checksum_file" "$checksum_url" >/dev/null 2>&1
    elif command_exists wget; then
        wget -qO- "$checksum_url" > "$checksum_file" 2>/dev/null
    fi
    
    if [ -f "$checksum_file" ]; then
        local expected_checksum=$(grep "$filename" "$checksum_file" | awk '{print $1}')
        if [ -n "$expected_checksum" ]; then
            local actual_checksum=$(sha256sum "$download_path" | awk '{print $1}')
            if [ "$expected_checksum" = "$actual_checksum" ]; then
                print_success "Checksum verified" >&2
            else
                print_error "Checksum verification failed" >&2
                print_error "Expected: $expected_checksum" >&2
                print_error "Actual:   $actual_checksum" >&2
                exit 1
            fi
        else
            print_warning "Could not find checksum for $filename, skipping verification" >&2
        fi
    else
        print_warning "Could not download checksums, skipping verification" >&2
    fi
    
    echo "$download_path"
}

# Install the binary
install_binary() {
    local archive_path="$1"
    local install_path="$INSTALL_DIR/benchctl"
    
    print_info "Extracting binary..."
    
    # Extract the archive
    local extract_dir=$(mktemp -d)
    tar -xzf "$archive_path" -C "$extract_dir"
    
    # Find the binary
    local binary_path=$(find "$extract_dir" -name "benchctl" -type f -executable | head -n1)
    
    if [ -z "$binary_path" ]; then
        print_error "Could not find benchctl binary in archive"
        exit 1
    fi
    
    # Check if install directory is writable
    if [ ! -w "$INSTALL_DIR" ]; then
        print_info "Install directory $INSTALL_DIR is not writable, using sudo..."
        sudo cp "$binary_path" "$install_path"
        sudo chmod +x "$install_path"
    else
        cp "$binary_path" "$install_path"
        chmod +x "$install_path"
    fi
    
    # Clean up
    rm -rf "$(dirname "$archive_path")"
    rm -rf "$extract_dir"
    
    print_success "benchctl installed successfully to $install_path"
}

# Verify installation
verify_installation() {
    if command_exists benchctl; then
        local installed_version=$(benchctl --version 2>/dev/null | head -n1 || echo "unknown")
        print_success "Installation verified"
        print_info "Installed version: $installed_version"
        print_info "Run 'benchctl --help' to get started"
    else
        print_warning "benchctl command not found in PATH"
        print_info "You may need to add $INSTALL_DIR to your PATH"
        print_info "Add this to your shell profile: export PATH=\"\$PATH:$INSTALL_DIR\""
    fi
}

show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "benchctl installer - Install benchctl CLI tool"
    echo ""
    echo "Options:"
    echo "  -v, --version VERSION    Install specific version (default: latest)"
    echo "  -d, --dir DIRECTORY      Install directory (default: /usr/local/bin)"
    echo "  -h, --help              Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  VERSION                  Version to install (default: latest)"
    echo "  INSTALL_DIR              Directory to install to (default: /usr/local/bin)"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Install latest version"
    echo "  $0 --version v0.1.4                  # Install specific version"
    echo "  $0 --dir ~/.local/bin                 # Install to custom directory"
    echo "  VERSION=v0.1.4 $0                     # Install specific version via env var"
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -d|--dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

main() {
    print_info "benchctl installer"
    print_info "Repository: https://github.com/$REPO"
    print_info "Install directory: $INSTALL_DIR"
    
    # Detect platform
    local platform=$(detect_platform)
    print_info "Detected platform: $platform"
    
    # Get version
    get_latest_version
    
    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        print_info "Creating install directory: $INSTALL_DIR"
        if [ ! -w "$(dirname "$INSTALL_DIR")" ]; then
            sudo mkdir -p "$INSTALL_DIR"
        else
            mkdir -p "$INSTALL_DIR"
        fi
    fi
    
    # Download and install
    local archive_path=$(download_and_verify "$platform" "$VERSION")
    install_binary "$archive_path"
    
    # Verify installation
    verify_installation
    
    print_success "Installation complete!"
}

# Run main function only if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
