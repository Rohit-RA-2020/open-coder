# Open Coder 🤖
![Example Screenshot](images/1.png)
![Example Screenshot](images/2.png)

A powerful AI coding agent that can interact with your codebase through natural language conversations while having full access to create, read, delete, and update files using the Model Context Protocol (MCP).

## 🌟 Features

- **Modern TUI Interface**: aesthetic split-pane terminal UI built with Bubble Tea
- **Interactive File Tree**: Browse and select files from a sidebar
- **Code Panel**: View and scroll through code files with syntax highlighting
- **Natural Language Interface**: Chat with the AI agent using plain English
- **File System Operations**: Complete CRUD (Create, Read, Update, Delete) operations on files
- **Model Context Protocol (MCP)**: Extensible tool system for adding new capabilities
- **Streaming Responses**: Real-time AI responses with tool execution feedback
- **Multi-server Support**: Connect to multiple MCP servers simultaneously
- **Themes**: Switch between Dark and Light modes on the fly
- **Codebase Indexing**: Use `/index` command to index your codebase for semantic search with vector embeddings
- **Auto-save Conversations**: Automatically save chat history to files
- **Tool Call Cancellation**: Cancel long-running tool calls with Ctrl+C without interrupting the conversation

## 📁 Project Structure

```
open-coder/
├── main.go                 # Main entry point
├── go.mod                 # Go module dependencies
├── go.sum                 # Dependency checksums
├── README.md              # This file
├── install.sh             # One-script installer (builds and installs everything)
├── pkg/                   # Reusable packages
│   └── indexer/          # Codebase indexing package
│       ├── config.go      # Configuration management
│       ├── scanner.go     # File discovery
│       ├── chunker.go     # File chunking router
│       ├── ast_chunker.go # AST-based semantic chunking (tree-sitter)
│       ├── indexer.go     # Main indexing orchestration
│       ├── config.example.go  # Example configurations
│       └── README.md      # Indexer documentation
├── ui/                    # TUI Implementation (Bubble Tea)
│   ├── model.go           # Main UI model
│   ├── view.go            # View rendering logic
│   ├── styles.go          # Lipgloss styles
│   ├── filetree.go        # Interactive file tree component
│   └── codepanel.go       # Code viewer component
└── tools/                 # MCP server tools directory
    ├── file-access/       # File operations MCP server
    │   ├── main.go        # Server implementation
    │   └── README.md      # Documentation
    ├── terminal/          # Terminal operations MCP server
    │   ├── main.go        # Server implementation
    │   └── README.md      # Documentation
    └── your-tool/         # Add your own MCP servers here!
        └── main.go        # Your custom MCP server
```

**✨ Dynamic Tool System**: Add any MCP server to the `tools/` directory and it will be automatically:
- Built by `install.sh`
- Installed to `~/.open-coder/`
- Connected by `main.go`

## 🔧 Tools & Capabilities

Open-Coder automatically discovers and connects to all MCP servers in the `tools/` directory.

### Built-in Tools

#### File Operations MCP Server (`tools/file-access/`)
Provides 8 comprehensive file and directory operations:
1. **`read_file`** - Read file contents with optional line ranges
2. **`read_line_range`** - Read specific lines or a range from a file
3. **`write_file`** - Create or overwrite files with content
4. **`edit_line_range`** - Edit specific lines or a range in a file
5. **`list_directory`** - List directory contents (with recursive option)
6. **`search_files`** - Find files by name patterns using glob syntax
7. **`search_content`** - Search text within files with context
8. **`delete_file`** - Delete files/directories (with recursive option)

#### Terminal Operations MCP Server (`tools/terminal/`)
Provides system command execution capabilities:
1. **`run_terminal_cmd`** - Execute system commands with arguments
2. **`run_terminal_cmd_with_input`** - Execute commands with stdin input

### Adding Custom Tools

**✨ Zero Configuration**: Simply add your MCP server to the `tools/` directory:

```
tools/
└── my-custom-tool/
    └── main.go  # Your MCP server implementation
```

**Example Tool Structure:**
```go
// tools/my-custom-tool/main.go
package main

import (
    "context"
    "fmt"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type CustomServer struct{}

func (c *CustomServer) MyCustomFunction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    return "Hello from my custom tool!", nil
}

func main() {
    server := &CustomServer{}

    mcpServer := mcp.NewServer(&mcp.Implementation{
        Name: "my-custom-tool",
        Version: "1.0.0",
    }, nil)

    // Add your tools here
    tool := &mcp.Tool{
        Name:        "my_custom_function",
        Description: "My custom functionality",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{},
        },
    }

    mcpServer.AddTool(tool, server.MyCustomFunction)

    // Server will listen for MCP protocol messages
    mcpServer.Listen(os.Stdin, os.Stdout)
}
```

**Automatic Discovery**: The install script and main application will automatically:
- Build your tool as `my-custom-tool-cli`
- Install it to `~/.open-coder/`
- Connect it as an MCP server named `my-custom-tool`
- Load all its available tools

**Result**: Your custom tool is now available in Open-Coder without any code changes!

## 🚀 Quick Start

### Prerequisites

- **For building**: Go 1.25.1 or later
- **For running**: OpenAI API key and terminal access
- **Standalone executable**: Once built and installed, no Go installation required

### 1. Clone and Setup

```bash
cd open-coder
```

### 2. Install Dependencies

```bash
go mod tidy
```

### 3. Install Open-Coder (One Command!)

```bash
# This builds AND installs everything automatically
./install.sh
```

The install script will:
- ✅ Check for Go installation
- ✅ **Auto-discover and build ALL tools** in the tools/ directory
- ✅ Install main application and all MCP servers to ~/.open-coder/
- ✅ Add to your PATH
- ✅ Configure environment variables

**✨ Dynamic Tool Discovery**: Any tool added to `tools/tool-name/main.go` will be automatically built and connected!

### 4. Run the Agent

```bash
open-coder
```

On first run, you'll be prompted to enter your OpenAI configuration:
- API Key
- Base URL
- Model

This configuration is automatically saved to `~/.open-coder/config` and won't need to be entered again.

You can also set environment variables to override the saved configuration:
```bash
export OPENAI_API_KEY="your-openai-api-key-here"
export OPENAI_BASE_URL="https://api.openai.com/v1"  # or your custom endpoint
export OPENAI_MODEL="gpt-4o-mini"  # or your preferred model
```

**Note**: The `/index` feature uses dedicated Azure OpenAI endpoints that are pre-configured in the code.

## 💬 Usage

The new interface features a modern TUI with split panes:
- **Left**: File Tree (browse directory structure)
- **Center**: Chat Interface (talk to the agent)
- **Right**: Code Panel (view selected files)

### ⌨️ Key Bindings

| Key | Action |
|-----|--------|
| `Tab` | Cycle focus (File Tree -> Chat -> Code Panel) |
| `Shift+Tab` | Reverse cycle focus |
| `F1` | Toggle Sidebar (File Tree) |
| `F2` | Toggle Code Panel |
| `Ctrl+T` | Toggle Theme (Light/Dark) |
| `Ctrl+L` | Clear Chat |
| `PgUp`/`PgDn` | Scroll Chat History |
| `Enter` | Send Message (in Chat) |
| `Ctrl+C` | Cancel Tool / Quit |

### Interactive Commands

- **`/settings`** - Open the interactive settings menu
- **`/index`** - Index the current codebase (indexes are now persisted!)
- **`/help`** - Show help menu
- **`/theme`** - Toggle color theme
- **`@`** - Open file browser (legacy command, now you can also just click or navigate the file tree)

### Codebase Indexing with `/index`

The `/index` command creates a semantic search index of your entire codebase:

```
You > /index

📂 Indexing codebase at: /home/user/project
📊 Indexing [████████░░] 80/100 - pkg/indexer/main.go
...
✅ Indexing complete! 15 files → 47 chunks
```

**New in v2.0**:
- **Persistence**: Indexes are saved to disk (`.open-coder-index`) and reused.
- **Progress**: Real-time progress bar UI.
- **Smart Updates**: Only re-indexes changed files (coming soon).

**How it works:**
1. **File Discovery**: Scans all code files.
2. **Smart Chunking**: AST-based (Go, JS/TS, Python) or line-based chunking.
3. **Summarization**: AI generates summaries with pseudo-code.
4. **Embedding**: Generates vector embeddings.
5. **Storage**: Stores vectors in Qdrant with rich metadata.

### Tool Call Cancellation

Open-Coder supports canceling long-running tool calls. You can interrupt the tool execution by pressing `Ctrl+C`. The agent will gracefully stop the tool and continue the conversation.

## ⚙️ Configuration

### Interactive Settings Menu

Access the settings menu by typing `/settings` during any conversation. You can customize:
- 🎨 **Appearance**: Colors for all UI elements
- 🖥️  **Display Options**: Timestamps, Hidden files
- 💾 **Chat Behavior**: Auto-save settings
- 🔌 **MCP Servers**: Manage connections
- ⚙️  **API Config**: Update OpenAI keys/models

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `OPENAI_API_KEY` | Your OpenAI API key | ✅ | - |
| `OPENAI_BASE_URL` | API endpoint URL | ✅ | - |
| `OPENAI_MODEL` | Model to use | ✅ | - |

## 🏗️ Architecture

### Core Components

1. **Agent Logic**: Manages OpenAI connection and tool execution loop.
2. **TUI (Bubble Tea)**: Handles the terminal user interface, rendering, and input.
3. **MCP Client**: Connects to MCP servers for capabilities (file access, terminal).
4. **Indexer**: Handles semantic code indexing and search.

### Data Flow

```mermaid
graph TD
    User[User Input] --> TUI[TUI (Bubble Tea)]
    TUI --> Agent[Agent Logic]
    Agent --> OpenAI[OpenAI API]
    OpenAI --> Agent
    Agent --> MCP[MCP Client]
    MCP --> Tools[Tools (File/Term)]
    Tools --> MCP
    MCP --> Agent
    Agent --> TUI
```

## 🔒 Security Notes

- File operations are executed in the current working directory.
- Recursive deletion operations can be dangerous.
- Always backup important files before using delete operations.

## 🛠️ Development

### Adding New MCP Servers
1. Create a new directory under `tools/`.
2. Implement the MCP server.
3. Run `./install.sh` to rebuild and connect.

### Testing
```bash
go test ./...
```

## 📚 Dependencies

- **charmbracelet/bubbletea**: Terminal UI framework
- **charmbracelet/lipgloss**: CSS for terminal
- **charmbracelet/glamour**: Markdown rendering
- **modelcontextprotocol/go-sdk**: MCP implementation
- **openai/openai-go**: OpenAI client
- **qdrant/go-client**: Vector database client

## 🤝 Contributing
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## 📄 License
MIT License

## 🆘 Troubleshooting
- **Rendering Issues**: Ensure your terminal supports true color (e.g., iTerm2, Alacritty, VS Code Terminal).
- **Font Issues**: A Nerd Font is recommended for icons (though standard unicode is used where possible).
