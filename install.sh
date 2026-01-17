#!/bin/bash

# Open-Coder Installation Script
# Builds and installs the open-coder CLI tool and MCP servers to ~/.open-coder
# Automatically configures PATH for immediate use

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$HOME/.open-coder"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

# Status indicators
print_status() {
    echo -e "${GREEN}[OK]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_step() {
    echo -e "\n${BOLD}==> $1${NC}"
}

# Function to create config.json from .env file
create_config_from_env() {
    local env_file="$SCRIPT_DIR/.env"
    local config_file="$INSTALL_DIR/config"

    if [[ ! -f "$env_file" ]]; then
        print_warning ".env file not found, skipping config creation"
        return 1
    fi

    local api_key=""
    local base_url=""
    local model=""
    local embedding_base_url=""
    local embedding_api_key=""
    local embedding_model=""
    local summary_base_url=""
    local summary_api_key=""
    local summary_model=""
    local qdrant_host=""
    local qdrant_port=""
    local chunk_size=""
    local chunk_overlap=""
    local vector_dimensions=""

    while IFS='=' read -r key value; do
        key=$(echo "$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
        value=$(echo "$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
        value=$(echo "$value" | sed 's/^"\(.*\)"$/\1/')
        value=$(echo "$value" | sed "s/^'\(.*\)'$/\1/")

        case "$key" in
            "OPENAI_API_KEY") api_key="$value" ;;
            "OPENAI_BASE_URL") base_url="$value" ;;
            "OPENAI_MODEL") model="$value" ;;
            "EMBEDDING_BASE_URL") embedding_base_url="$value" ;;
            "EMBEDDING_API_KEY") embedding_api_key="$value" ;;
            "EMBEDDING_MODEL") embedding_model="$value" ;;
            "SUMMARY_BASE_URL") summary_base_url="$value" ;;
            "SUMMARY_API_KEY") summary_api_key="$value" ;;
            "SUMMARY_MODEL") summary_model="$value" ;;
            "QDRANT_HOST") qdrant_host="$value" ;;
            "QDRANT_PORT") qdrant_port="$value" ;;
            "CHUNK_SIZE") chunk_size="$value" ;;
            "CHUNK_OVERLAP") chunk_overlap="$value" ;;
            "VECTOR_DIMENSIONS") vector_dimensions="$value" ;;
        esac
    done < "$env_file"

    if [[ -z "$api_key" || -z "$base_url" || -z "$model" ]]; then
        print_warning "Missing OpenAI credentials in .env file"
        print_info "Required: OPENAI_API_KEY, OPENAI_BASE_URL, OPENAI_MODEL"
        return 1
    fi

    cat > "$config_file" <<EOF
{
  "api_key": "$api_key",
  "base_url": "$base_url",
  "model": "$model",
  "indexer": {
    "embedding": {
      "base_url": "$embedding_base_url",
      "api_key": "$embedding_api_key",
      "model": "$embedding_model"
    },
    "summary": {
      "base_url": "$summary_base_url",
      "api_key": "$summary_api_key",
      "model": "$summary_model"
    },
    "qdrant": {
      "host": "$qdrant_host",
      "port": "$qdrant_port"
    },
    "chunking": {
      "size": "$chunk_size",
      "overlap": "$chunk_overlap"
    },
    "vector_dimensions": "$vector_dimensions"
  }
}
EOF

    chmod 600 "$config_file"
    print_status "Configuration created at $config_file"
    print_info "API Key: ${api_key:0:8}****${api_key: -4}"
    print_info "Base URL: $base_url"
    print_info "Model: $model"
}

# Track built binaries for cleanup
declare -a BUILT_BINARIES

# Function to clean up build artifacts from source directory
cleanup_build_artifacts() {
    print_step "Cleaning build artifacts from source directory"
    
    local cleaned=0
    for binary in "${BUILT_BINARIES[@]}"; do
        if [[ -f "$binary" ]]; then
            rm -f "$binary"
            cleaned=$((cleaned + 1))
        fi
    done
    
    if [[ $cleaned -gt 0 ]]; then
        print_status "Removed $cleaned build artifact(s) from source directory"
    else
        print_info "No build artifacts to clean"
    fi
}

echo -e "${BOLD}Open-Coder Installation${NC}"
echo "========================"

# Check prerequisites
print_step "Checking prerequisites"

if ! command -v go &> /dev/null; then
    print_error "Go is not installed"
    print_info "Install from: https://golang.org/doc/install"
    exit 1
fi

print_status "Go $(go version | awk '{print $3}')"

# Build main application
print_step "Building main application"

if [[ ! -f "$SCRIPT_DIR/main.go" ]]; then
    print_error "main.go not found at $SCRIPT_DIR/main.go"
    exit 1
fi

cd "$SCRIPT_DIR"
if go build -o open-coder main.go; then
    print_status "open-coder"
    BUILT_BINARIES+=("$SCRIPT_DIR/open-coder")
else
    print_error "Failed to build main application"
    exit 1
fi

# Build MCP servers
print_step "Building MCP servers"

tools_found=0
while IFS= read -r -d '' tool_dir; do
    tool_name=$(basename "$tool_dir")
    main_go_path="$tool_dir/main.go"

    if [[ -f "$main_go_path" ]]; then
        cd "$tool_dir"
        binary_name="${tool_name}-cli"

        if go build -o "$binary_name" main.go; then
            print_status "$binary_name"
            BUILT_BINARIES+=("$tool_dir/$binary_name")
            tools_found=$((tools_found + 1))
        else
            print_error "Failed to build $tool_name"
            exit 1
        fi
    fi
done < <(find "$SCRIPT_DIR/tools" -mindepth 1 -maxdepth 1 -type d -print0)

if [[ $tools_found -eq 0 ]]; then
    print_error "No tools found in $SCRIPT_DIR/tools"
    exit 1
fi

cd "$SCRIPT_DIR"

# Check for .env configuration
print_step "Checking configuration"

if [[ -f "$SCRIPT_DIR/.env" ]]; then
    print_status "Found .env file"
    ENV_FILE_EXISTS=true
else
    print_warning "No .env file found"
    ENV_FILE_EXISTS=false
fi

# Install binaries
print_step "Installing to $INSTALL_DIR"

mkdir -p "$INSTALL_DIR"

cp "$SCRIPT_DIR/open-coder" "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/open-coder"
print_status "Installed open-coder"

copied_servers=0
while IFS= read -r -d '' tool_dir; do
    tool_name=$(basename "$tool_dir")
    binary_name="${tool_name}-cli"
    binary_path="$tool_dir/$binary_name"

    if [[ -f "$binary_path" ]]; then
        cp "$binary_path" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/$binary_name"
        print_status "Installed $binary_name"
        copied_servers=$((copied_servers + 1))
    fi
done < <(find "$SCRIPT_DIR/tools" -mindepth 1 -maxdepth 1 -type d -print0)

# Create configuration
if [[ "$ENV_FILE_EXISTS" == "true" ]]; then
    print_step "Creating configuration"
    if ! create_config_from_env; then
        print_error "Failed to create configuration"
        exit 1
    fi
else
    if [[ -f "$SCRIPT_DIR/.env.example" ]]; then
        cp "$SCRIPT_DIR/.env.example" "$INSTALL_DIR/"
        print_info "Template copied to $INSTALL_DIR/.env.example"
    fi
fi

# Clean up build artifacts
cleanup_build_artifacts

# Configure PATH
print_step "Configuring environment"

add_to_shell_config() {
    local shell_config_file="$1"
    local export_line="export PATH=\"\$HOME/.open-coder:\$PATH\""
    local file_ops_export="export OPEN_CODER_FILE_OPS_PATH=\"\$HOME/.open-coder/file-access-cli\""
    local terminal_export="export OPEN_CODER_TERMINAL_PATH=\"\$HOME/.open-coder/terminal-cli\""
    local config_file_export="export OPEN_CODER_CONFIG_FILE=\"\$HOME/.open-coder/config\""

    if [[ ! -f "$shell_config_file" ]]; then
        touch "$shell_config_file"
    fi

    if ! grep -q "open-coder" "$shell_config_file"; then
        {
            echo ""
            echo "# Open-Coder"
            echo "$export_line"
            echo "$file_ops_export"
            echo "$terminal_export"
            echo "$config_file_export"
            echo ""
        } >> "$shell_config_file"
        print_status "Updated $shell_config_file"
    else
        print_info "PATH already configured in $shell_config_file"
    fi
}

CURRENT_SHELL=$(basename "$SHELL")
case "$CURRENT_SHELL" in
    zsh)  SHELL_CONFIG="$HOME/.zshrc" ;;
    bash) SHELL_CONFIG="$HOME/.bashrc" ;;
    *)
        print_warning "Unknown shell: $CURRENT_SHELL"
        print_info "Add manually: export PATH=\"\$HOME/.open-coder:\$PATH\""
        SHELL_CONFIG=""
        ;;
esac

if [[ -n "$SHELL_CONFIG" ]]; then
    add_to_shell_config "$SHELL_CONFIG"
fi

if [[ -f "$HOME/.profile" ]]; then
    add_to_shell_config "$HOME/.profile"
fi

# Summary
echo
echo -e "${BOLD}Installation Complete${NC}"
echo "====================="
echo
echo "Installed components:"
echo "  - open-coder (main CLI)"
echo "  - $copied_servers MCP server(s)"
echo
echo "To start using Open-Coder:"
echo "  1. Restart your terminal, or run: source $SHELL_CONFIG"
echo "  2. Run: open-coder"
echo
if [[ "$ENV_FILE_EXISTS" == "true" ]]; then
    echo "Configuration: ~/.open-coder/config"
else
    echo "Configuration required. Options:"
    echo "  - Create ~/.open-coder/.env from the template and re-run install"
    echo "  - Set environment variables: OPENAI_API_KEY, OPENAI_BASE_URL, OPENAI_MODEL"
fi
echo
echo "Documentation: README.md"
