#!/bin/bash

# Open-Coder Complete Installation Script
# This script builds and installs the open-coder CLI tool and its dependencies to your home directory
# and adds it to your PATH so you can run it from anywhere.
# ONE SCRIPT DOES IT ALL - no manual building required!

set -e

echo "🔧 Installing Open-Coder CLI Tool..."
echo

# Get the directory where this script is located (the project root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$HOME/.open-coder"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✅${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠️ ${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ️ ${NC} $1"
}

# Function to create config.json from .env file
create_config_from_env() {
    local env_file="$SCRIPT_DIR/.env"
    local config_file="$INSTALL_DIR/config"

    # Check if .env file exists
    if [[ ! -f "$env_file" ]]; then
        print_warning ".env file not found, skipping config creation"
        return 1
    fi

    # Initialize all configuration values
    local api_key=""
    local base_url=""
    local model=""

    # Indexer configuration values
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

    # Read .env file line by line
    while IFS='=' read -r key value; do
        # Remove whitespace
        key=$(echo "$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
        value=$(echo "$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

        # Remove surrounding quotes if present
        value=$(echo "$value" | sed 's/^"\(.*\)"$/\1/')
        value=$(echo "$value" | sed "s/^'\(.*\)'$/\1/")

        case "$key" in
            "OPENAI_API_KEY")
                api_key="$value"
                ;;
            "OPENAI_BASE_URL")
                base_url="$value"
                ;;
            "OPENAI_MODEL")
                model="$value"
                ;;
            "EMBEDDING_BASE_URL")
                embedding_base_url="$value"
                ;;
            "EMBEDDING_API_KEY")
                embedding_api_key="$value"
                ;;
            "EMBEDDING_MODEL")
                embedding_model="$value"
                ;;
            "SUMMARY_BASE_URL")
                summary_base_url="$value"
                ;;
            "SUMMARY_API_KEY")
                summary_api_key="$value"
                ;;
            "SUMMARY_MODEL")
                summary_model="$value"
                ;;
            "QDRANT_HOST")
                qdrant_host="$value"
                ;;
            "QDRANT_PORT")
                qdrant_port="$value"
                ;;
            "CHUNK_SIZE")
                chunk_size="$value"
                ;;
            "CHUNK_OVERLAP")
                chunk_overlap="$value"
                ;;
            "VECTOR_DIMENSIONS")
                vector_dimensions="$value"
                ;;
        esac
    done < "$env_file"

    # Check if we have the required OpenAI credentials
    if [[ -z "$api_key" || -z "$base_url" || -z "$model" ]]; then
        print_warning "Missing OpenAI credentials in .env file"
        print_info "Required: OPENAI_API_KEY, OPENAI_BASE_URL, OPENAI_MODEL"
        return 1
    fi

    # Create comprehensive JSON config
    cat > "$config_file" << EOF
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

    # Set secure permissions on config file
    chmod 600 "$config_file"

    print_status "Complete configuration created at $config_file"
    print_info "OpenAI API Key: ${api_key:0:8}****${api_key: -4}"
    print_info "OpenAI Base URL: $base_url"
    print_info "OpenAI Model: $model"
    if [[ -n "$embedding_base_url" ]]; then
        print_info "Indexer configured with embedding, summary, and Qdrant settings"
    fi
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go first."
    print_info "Visit: https://golang.org/doc/install"
    exit 1
fi

print_status "Go found, starting build process..."

# Build main application
echo
echo "🔨 Building main application..."
if [[ -f "$SCRIPT_DIR/main.go" ]]; then
    cd "$SCRIPT_DIR"
    if go build -o open-coder main.go; then
        print_status "Main application built successfully"
    else
        print_error "Failed to build main application"
        exit 1
    fi
else
    print_error "main.go not found at $SCRIPT_DIR/main.go"
    exit 1
fi

# Build file operations MCP server
echo
echo "🔨 Building file operations MCP server..."
if [[ -f "$SCRIPT_DIR/tools/file-access/main.go" ]]; then
    cd "$SCRIPT_DIR/tools/file-access"
    if go build -o file-ops-cli main.go; then
        print_status "File operations server built successfully"
    else
        print_error "Failed to build file operations server"
        exit 1
    fi
else
    print_error "File operations main.go not found at $SCRIPT_DIR/tools/file-access/main.go"
    exit 1
fi

# Scan and build all tools in tools directory
echo
echo "🔍 Scanning tools directory for MCP servers..."

# Find all subdirectories in tools/
tools_found=0
while IFS= read -r -d '' tool_dir; do
    tool_name=$(basename "$tool_dir")
    main_go_path="$tool_dir/main.go"

    if [[ -f "$main_go_path" ]]; then
        echo
        echo "🔨 Building $tool_name MCP server..."
        cd "$tool_dir"
        binary_name="${tool_name}-cli"

        if go build -o "$binary_name" main.go; then
            print_status "$tool_name server built successfully"
            tools_found=$((tools_found + 1))
        else
            print_error "Failed to build $tool_name server"
            exit 1
        fi
    fi
done < <(find "$SCRIPT_DIR/tools" -mindepth 1 -maxdepth 1 -type d -print0)

if [[ $tools_found -eq 0 ]]; then
    print_error "No tools found in $SCRIPT_DIR/tools directory"
    print_info "Tools should be organized as: tools/tool-name/main.go"
    exit 1
fi

print_status "Found and built $tools_found MCP server(s)"
echo
echo "📋 Built tools:"
while IFS= read -r -d '' tool_dir; do
    tool_name=$(basename "$tool_dir")
    binary_name="${tool_name}-cli"
    if [[ -f "$tool_dir/$binary_name" ]]; then
        echo "  ✅ $tool_name → $binary_name"
    fi
done < <(find "$SCRIPT_DIR/tools" -mindepth 1 -maxdepth 1 -type d -print0)

# Return to project root
cd "$SCRIPT_DIR"
print_status "All binaries built successfully, proceeding with installation..."

# Handle .env file if it exists
echo
echo "🔧 Checking for .env configuration..."
if [[ -f "$SCRIPT_DIR/.env" ]]; then
    print_status "Found .env file, will be copied to installation directory"
    ENV_FILE_EXISTS=true
else
    print_warning "No .env file found"
    print_info "You can create a .env file based on .env.example for configuration"
    ENV_FILE_EXISTS=false
fi

# Create installation directory
echo
echo "📁 Creating installation directory at $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"

# Copy main application
echo
echo "📋 Installing main application..."
cp "$SCRIPT_DIR/open-coder" "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/open-coder"

# Create configuration from .env file if it exists
if [[ "$ENV_FILE_EXISTS" == "true" ]]; then
    echo
    echo "🔧 Creating configuration from .env file..."
    if create_config_from_env; then
        print_status "Configuration created successfully from .env file"
    else
        print_error "Failed to create configuration from .env file"
        exit 1
    fi
else
    # Copy .env.example as a template for users
    echo
    echo "📋 Installing .env.example template..."
    cp "$SCRIPT_DIR/.env.example" "$INSTALL_DIR/"
    print_status ".env.example template copied to $INSTALL_DIR/.env.example"
fi

# Copy all MCP server binaries
echo
echo "📋 Installing MCP servers..."
copied_servers=0
while IFS= read -r -d '' tool_dir; do
    tool_name=$(basename "$tool_dir")
    binary_name="${tool_name}-cli"
    binary_path="$tool_dir/$binary_name"

    if [[ -f "$binary_path" ]]; then
        cp "$binary_path" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/$binary_name"
        copied_servers=$((copied_servers + 1))
    fi
done < <(find "$SCRIPT_DIR/tools" -mindepth 1 -maxdepth 1 -type d -print0)

print_status "Installed main application and $copied_servers MCP server(s)"

# Detect shell and setup PATH
echo
echo "🔧 Setting up PATH..."

# Function to add to shell configuration
add_to_shell_config() {
    local shell_config_file="$1"
    local export_line="export PATH=\"\$HOME/.open-coder:\$PATH\""
    local file_ops_export="export OPEN_CODER_FILE_OPS_PATH=\"\$HOME/.open-coder/file-ops-cli\""
    local terminal_export="export OPEN_CODER_TERMINAL_PATH=\"\$HOME/.open-coder/terminal-cli\""
    local config_file_export="export OPEN_CODER_CONFIG_FILE=\"\$HOME/.open-coder/config\""

    if [[ ! -f "$shell_config_file" ]]; then
        echo "Creating $shell_config_file..."
        touch "$shell_config_file"
    fi

    # Check if the PATH is already added
    if ! grep -q "open-coder" "$shell_config_file"; then
        echo "" >> "$shell_config_file"
        echo "# Added by Open-Coder installation" >> "$shell_config_file"
        echo "$export_line" >> "$shell_config_file"
        echo "$file_ops_export" >> "$shell_config_file"
        echo "$terminal_export" >> "$shell_config_file"
        echo "$config_file_export" >> "$shell_config_file"
        echo "" >> "$shell_config_file"
        print_status "Added Open-Coder to $shell_config_file"
    else
        print_warning "Open-Coder appears to already be in $shell_config_file"
    fi
}

# Detect the current shell and its configuration file
CURRENT_SHELL=$(basename "$SHELL")
if [[ "$CURRENT_SHELL" == "zsh" ]]; then
    SHELL_CONFIG="$HOME/.zshrc"
elif [[ "$CURRENT_SHELL" == "bash" ]]; then
    SHELL_CONFIG="$HOME/.bashrc"
else
    print_warning "Unknown shell: $CURRENT_SHELL"
    print_warning "Please manually add the following to your shell configuration:"
    echo "  export PATH=\"\$HOME/.open-coder:\$PATH\""
    echo "  export OPEN_CODER_FILE_OPS_PATH=\"\$HOME/.open-coder/file-ops-cli\""
    echo "  export OPEN_CODER_TERMINAL_PATH=\"\$HOME/.open-coder/terminal-cli\""
    echo "  export OPEN_CODER_CONFIG_FILE=\"\$HOME/.open-coder/config\""
    SHELL_CONFIG=""
fi

if [[ -n "$SHELL_CONFIG" ]]; then
    add_to_shell_config "$SHELL_CONFIG"
fi

# Also add to .profile as a fallback for login shells
if [[ -f "$HOME/.profile" ]]; then
    add_to_shell_config "$HOME/.profile"
fi

echo
echo "🎉 Installation completed successfully!"
echo
echo "🚀 ONE-SCRIPT SETUP COMPLETE!"
echo "✅ Built all binaries from source"
echo "✅ Installed to ~/.open-coder/"
echo "✅ Added to your PATH"
echo "✅ Auto-discovered and installed $copied_servers MCP server(s)"
echo "✅ Ready for automatic tool discovery"
echo
echo "To start using Open-Coder, you have two options:"
echo
echo "Option 1 - Restart your terminal:"
echo "  Close and reopen your terminal, then run:"
echo "  open-coder"
echo
echo "Option 2 - Reload your shell configuration:"
echo "  source ~/.bashrc  # or source ~/.zshrc"
echo "  Then run:"
echo "  open-coder"
echo
echo "📋 Configuration Setup"
echo
if [[ "$ENV_FILE_EXISTS" == "true" ]]; then
    echo "✅ Configuration extracted from .env file and saved to ~/.open-coder/config"
    echo "✅ All OpenAI and indexer settings are now available in the config file"
    echo "The main CLI tool will use the credentials from ~/.open-coder/config"
    echo "The indexer will use the settings from ~/.open-coder/config"
else
    echo "⚠️  No .env file found"
    echo "✅ .env.example template installed to ~/.open-coder/.env.example"
    echo ""
    echo "To configure both the CLI and indexer, copy and edit the template:"
    echo "  cp ~/.open-coder/.env.example ~/.open-coder/.env"
    echo "  # Edit ~/.open-coder/.env with your actual values"
    echo "  # Then run the installation again"
    echo ""
    echo "To configure the main CLI tool only, you can:"
    echo "  • Set environment variables: export OPENAI_API_KEY=\"your-key\""
    echo "  • Run the CLI tool once to be prompted for configuration"
fi
echo
echo "Environment variables supported by the indexer:"
echo "  EMBEDDING_BASE_URL, EMBEDDING_API_KEY, EMBEDDING_MODEL"
echo "  SUMMARY_BASE_URL, SUMMARY_API_KEY, SUMMARY_MODEL"
echo "  QDRANT_HOST, QDRANT_PORT, VECTOR_DIMENSIONS"
echo "  CHUNK_SIZE, CHUNK_OVERLAP"
echo
echo "For the main CLI tool, you can set:"
echo "  export OPENAI_API_KEY=\"your-api-key-here\""
echo "  export OPENAI_BASE_URL=\"https://api.openai.com/v1\""
echo "  export OPENAI_MODEL=\"gpt-4o-mini\""
echo ""
echo "Or create a .env file in your project directory with these values"
echo "and run the installation script again to automatically configure everything."
echo
echo "📚 For more information, see the README.md file"
echo
print_status "Open-Coder is now fully installed and ready to use! 🚀"
