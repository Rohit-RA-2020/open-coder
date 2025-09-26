# Open-Coder Installation Guide

This guide will help you install Open-Coder as a standalone command-line tool that can be run from anywhere without requiring Go to be installed.

## 🚀 Quick Installation (One Command!)

**Just run the install script - it builds AND installs everything:**

```bash
cd /Users/shivanshi/Documents/open-coder
./install.sh
```

The install script automatically:
- ✅ Checks for Go installation
- ✅ **Auto-discovers and builds ALL tools** in the tools/ directory
- ✅ Installs main application and all MCP servers to ~/.open-coder/
- ✅ Adds to your PATH
- ✅ Configures environment variables

**✨ Dynamic Installation**: Any tool added to `tools/tool-name/main.go` will be automatically built and installed!

## 🔄 Next Steps

1. **Reload your shell configuration**:
   ```bash
   source ~/.zshrc  # or source ~/.bashrc
   ```

2. **You're done!** Now you can run Open-Coder from anywhere:
   ```bash
   open-coder
   ```

   On first run, it will prompt you for your OpenAI configuration, which will be saved automatically.

## 📁 What the Installation Does

The installation script:

1. **Creates installation directory**: `~/.open-coder/`
2. **Auto-discovers tools**: Scans `tools/` directory for any MCP servers
3. **Builds all tools**: Compiles each tool as `{tool-name}-cli`
4. **Installs everything**: Copies main app and all tool binaries
5. **Sets permissions**: Makes all binaries executable
6. **Updates PATH**: Adds `~/.open-coder/` to your PATH
7. **Enables auto-discovery**: Main app will automatically find and connect all tools

**✨ Dynamic System**: Any tool in `tools/tool-name/main.go` gets automatically built and connected!

### Adding Custom Tools

**Zero Configuration**: Add your MCP server to the `tools/` directory:

```
tools/
└── my-custom-tool/
    └── main.go  # Your MCP server implementation
```

**Example Structure:**
```go
// tools/my-custom-tool/main.go
package main

import (
    "context"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type CustomServer struct{}

func (c *CustomServer) MyFunction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    return "Hello from my custom tool!", nil
}

func main() {
    server := &CustomServer{}
    mcpServer := mcp.NewServer(&mcp.Implementation{
        Name: "my-custom-tool",
        Version: "1.0.0",
    }, nil)

    // Add your tools...
    mcpServer.Listen(os.Stdin, os.Stdout)
}
```

**Result**: Your custom tool is automatically built, installed, and connected!

## 📋 Prerequisites

- **Go**: Required only for installation (to build the binaries)
- **OpenAI API Key**: Required for running the application
- **Terminal access**: For the interactive interface

## 📋 Configuration

Open-Coder will automatically manage your OpenAI configuration:

- **First run**: Prompts for API key, base URL, and model, then saves to `~/.open-coder/config`
- **Subsequent runs**: Uses saved configuration automatically
- **Environment override**: You can still override with environment variables:
  - `OPENAI_API_KEY`
  - `OPENAI_BASE_URL`
  - `OPENAI_MODEL`

## 🎯 Usage

Once installed, run Open-Coder from anywhere:

```bash
export OPENAI_API_KEY="your-openai-api-key-here"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export OPENAI_MODEL="gpt-4o-mini"
```

You can add these to your shell configuration file (`.zshrc`, `.bashrc`, or `.profile`) to make them permanent.

## 🧪 Testing the Installation

After installation, you can test that everything is working:

1. **Check if open-coder is in PATH**:
   ```bash
   which open-coder
   ```

2. **Test the binary**:
   ```bash
   open-coder
   ```
   (This will prompt for API key configuration)

3. **Check environment variables**:
   ```bash
   echo $OPEN_CODER_FILE_OPS_PATH
   echo $OPEN_CODER_TERMINAL_PATH
   ```

## 🔧 Manual Installation (Alternative)

If you prefer to install manually:

1. **Create directory**:
   ```bash
   mkdir -p ~/.open-coder
   ```

2. **Copy binaries**:
   ```bash
   cp open-coder ~/.open-coder/
   cp tools/file-access/file-ops-cli ~/.open-coder/
   cp tools/terminal/terminal-cli ~/.open-coder/
   ```

3. **Make executable**:
   ```bash
   chmod +x ~/.open-coder/*
   ```

4. **Add to PATH** (add to your shell config):
   ```bash
   export PATH="$HOME/.open-coder:$PATH"
   export OPEN_CODER_FILE_OPS_PATH="$HOME/.open-coder/file-ops-cli"
   export OPEN_CODER_TERMINAL_PATH="$HOME/.open-coder/terminal-cli"
   ```

## 🛠️ Troubleshooting

### "command not found: open-coder"
- Reload your shell configuration: `source ~/.zshrc`
- Or restart your terminal
- Check if `~/.open-coder/` exists and contains the binaries

### "Failed to connect to file-ops server"
- Check if the environment variables are set correctly
- Verify the binaries exist at the paths specified by the environment variables
- Try running: `ls -la ~/.open-coder/`

### Permission issues
- Make sure the binaries are executable: `chmod +x ~/.open-coder/*`
- Check that you have read/write permissions in the installation directory

## 📚 Files Created

- `~/.open-coder/open-coder` - Main application binary
- `~/.open-coder/file-ops-cli` - File operations MCP server
- `~/.open-coder/terminal-cli` - Terminal operations MCP server
- Updated shell configuration files with PATH and environment variables

## 🎯 You're Ready!

Once installed, you can use Open-Coder from any directory:

```bash
# Navigate to any project directory
cd /path/to/your/project

# Start Open-Coder
open-coder

# Use the interactive chat interface
# Type '/settings' to customize appearance, configuration, and more
# Use '@' to browse and reference files
# Type 'exit' or 'quit' to end the session
```

Happy coding with your AI assistant! 🚀
