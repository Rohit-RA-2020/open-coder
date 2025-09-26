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

# Create installation directory
echo
echo "📁 Creating installation directory at $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"

# Copy main application
echo
echo "📋 Installing main application..."
cp "$SCRIPT_DIR/open-coder" "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/open-coder"

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
echo "On first run, Open-Coder will prompt you for:"
echo "- API Key"
echo "- Base URL"
echo "- Model"
echo
echo "This configuration will be automatically saved to ~/.open-coder/config"
echo "and won't need to be entered again on subsequent runs."
echo
echo "Alternatively, you can set environment variables to override the saved config:"
echo "  export OPENAI_API_KEY=\"your-api-key-here\""
echo "  export OPENAI_BASE_URL=\"https://api.openai.com/v1\""
echo "  export OPENAI_MODEL=\"gpt-4o-mini\""
echo
echo "📚 For more information, see the README.md file"
echo
print_status "Open-Coder is now fully installed and ready to use! 🚀"
