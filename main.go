package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
)

// Config represents the application configuration
type Config struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

// getConfigPath returns the path to the configuration file
func getConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "~" // fallback
	}
	return filepath.Join(homeDir, ".open-coder", "config")
}

// loadConfig reads configuration from file
func loadConfig() (*Config, error) {
	configPath := getConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// saveConfig writes configuration to file
func saveConfig(config *Config) error {
	configPath := getConfigPath()

	// Create directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfiguration gets configuration from environment variables, config file, or prompts user
func getConfiguration() (*Config, error) {
	// First priority: environment variables
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	baseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))

	// If all environment variables are set, use them
	if apiKey != "" && baseURL != "" && model != "" {
		return &Config{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		}, nil
	}

	// Second priority: config file
	config, err := loadConfig()
	if err == nil {
		// Override with environment variables if they exist
		if apiKey != "" {
			config.APIKey = apiKey
		}
		if baseURL != "" {
			config.BaseURL = baseURL
		}
		if model != "" {
			config.Model = model
		}
		return config, nil
	}

	// Third priority: prompt user (first time setup)
	pterm.FgLightYellow.Println("🔧 First-time setup - Please provide your OpenAI configuration:")
	pterm.FgLightWhite.Println("This will be saved to ~/.open-coder/config for future use.")
	pterm.FgLightWhite.Println("You can also set these as environment variables to override the saved config.")
	pterm.FgLightWhite.Println()

	reader := bufio.NewReader(os.Stdin)

	// Prompt for API key if not set
	if apiKey == "" {
		pterm.FgLightWhite.Print("API Key: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read API key: %w", err)
		}
		apiKey = strings.TrimSpace(input)
	}

	// Prompt for base URL if not set
	if baseURL == "" {
		pterm.FgLightWhite.Print("Base URL: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read base URL: %w", err)
		}
		baseURL = strings.TrimSpace(input)
	}

	// Prompt for model if not set
	if model == "" {
		pterm.FgLightWhite.Print("Model: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read model: %w", err)
		}
		model = strings.TrimSpace(input)
	}

	config = &Config{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
	}

	// Save configuration for future use
	if err := saveConfig(config); err != nil {
		pterm.FgLightYellow.Printf("⚠️  Warning: Could not save configuration: %v\n", err)
		pterm.FgLightYellow.Println("You'll need to provide configuration on each run or set environment variables.")
	} else {
		pterm.FgLightGreen.Println("✅ Configuration saved! You won't be prompted again.")
	}

	return config, nil
}

// Configuration is sourced from the current environment:
// - OPENAI_API_KEY
// - OPENAI_BASE_URL
// - OPENAI_MODEL

type SimpleAgent struct {
	ctx            context.Context
	mcpClient      *mcp.Client
	servers        []*MCPServerConfig
	openaiClient   *openai.Client
	model          string
	apiKey         string // Store API key for settings access
	baseURL        string // Store base URL for settings access
	userID         string
	systemPrompt   string
	messages       []openai.ChatCompletionMessageParamUnion
	tools          []openai.ChatCompletionToolUnionParam
	assistantColor string // Color for assistant text output
	userColor      string // Color for user input text
	systemColor    string // Color for system messages
	toolColor      string // Color for tool output
	errorColor     string // Color for error messages
	showTimestamps bool   // Show timestamps in messages
	autoSaveChat   bool   // Auto-save conversations
	compactMode    bool   // Compact display mode
	currentDir     string // Current working directory for file browser
	showHidden     bool   // Show hidden files in file browser
}

type MCPServerConfig struct {
	Name    string
	Command string
	Args    []string
	Session *mcp.ClientSession
}

func NewSimpleAgent(ctx context.Context, model string, apiKey string, baseURL string) *SimpleAgent {
	openaiClient := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	return &SimpleAgent{
		ctx:            ctx,
		mcpClient:      mcp.NewClient(&mcp.Implementation{Name: "simple-agent", Version: "v1.0.0"}, nil),
		servers:        make([]*MCPServerConfig, 0),
		openaiClient:   &openaiClient,
		model:          model,
		apiKey:         apiKey,    // Store API key for settings access
		baseURL:        baseURL,   // Store base URL for settings access
		userID:         "user123", // Simple user ID for demo
		messages:       make([]openai.ChatCompletionMessageParamUnion, 0),
		tools:          make([]openai.ChatCompletionToolUnionParam, 0),
		assistantColor: "FgLightCyan",  // Default color for assistant text
		userColor:      "FgLightWhite", // Default color for user text
		systemColor:    "FgLightBlue",  // Default color for system messages
		toolColor:      "FgLightGreen", // Default color for tool output
		errorColor:     "FgLightRed",   // Default color for errors
		showTimestamps: false,          // Don't show timestamps by default
		autoSaveChat:   false,          // Don't auto-save by default
		compactMode:    false,          // Normal display mode by default
		currentDir:     "",             // Will be set to current working directory
		showHidden:     false,          // Don't show hidden files by default
	}
}

// InitConversation initializes a new conversation with a system prompt.
func (a *SimpleAgent) InitConversation(system string) {
	a.systemPrompt = system
	a.messages = []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(system)}
}

// getColorStyle returns the pterm color style for any stored color preference
func (a *SimpleAgent) getColorStyle(colorName string) pterm.Color {
	switch colorName {
	case "FgLightCyan":
		return pterm.FgLightCyan
	case "FgCyan":
		return pterm.FgCyan
	case "FgLightBlue":
		return pterm.FgLightBlue
	case "FgBlue":
		return pterm.FgBlue
	case "FgLightGreen":
		return pterm.FgLightGreen
	case "FgGreen":
		return pterm.FgGreen
	case "FgLightYellow":
		return pterm.FgLightYellow
	case "FgYellow":
		return pterm.FgYellow
	case "FgLightRed":
		return pterm.FgLightRed
	case "FgRed":
		return pterm.FgRed
	case "FgLightMagenta":
		return pterm.FgLightMagenta
	case "FgMagenta":
		return pterm.FgMagenta
	case "FgLightWhite":
		return pterm.FgLightWhite
	case "FgWhite":
		return pterm.FgWhite
	case "FgBlack":
		return pterm.FgBlack
	case "FgGray":
		return pterm.FgGray
	default:
		return pterm.FgLightCyan // Default fallback
	}
}

// getAssistantColorStyle returns the pterm color style for assistant text
func (a *SimpleAgent) getAssistantColorStyle() pterm.Color {
	return a.getColorStyle(a.assistantColor)
}

// getUserColorStyle returns the pterm color style for user input text
func (a *SimpleAgent) getUserColorStyle() pterm.Color {
	return a.getColorStyle(a.userColor)
}

// getSystemColorStyle returns the pterm color style for system messages
func (a *SimpleAgent) getSystemColorStyle() pterm.Color {
	return a.getColorStyle(a.systemColor)
}

// getToolColorStyle returns the pterm color style for tool output
func (a *SimpleAgent) getToolColorStyle() pterm.Color {
	return a.getColorStyle(a.toolColor)
}

// getErrorColorStyle returns the pterm color style for error messages
func (a *SimpleAgent) getErrorColorStyle() pterm.Color {
	return a.getColorStyle(a.errorColor)
}

// showSettingsMenu displays an interactive settings menu for the user
func (a *SimpleAgent) showSettingsMenu() error {
	for {
		pterm.FgLightWhite.Println("\n" + strings.Repeat("═", 50))
		pterm.FgLightCyan.Println("⚙️  SETTINGS")
		pterm.FgLightWhite.Println(strings.Repeat("─", 50))

		pterm.FgLightWhite.Println("Choose a category:")
		pterm.FgLightWhite.Println("1. 🎨 Appearance (Colors)")
		pterm.FgLightWhite.Println("2. 🖥️  Display Options")
		pterm.FgLightWhite.Println("3. 💾 Chat Behavior")
		pterm.FgLightWhite.Println("4. 🔌 MCP Server Settings")
		pterm.FgLightWhite.Println("5. ⚙️  Configuration (API, URL, Model)")
		pterm.FgLightWhite.Println("\n0. Back to Chat")
		pterm.FgLightWhite.Println(strings.Repeat("─", 50))

		pterm.FgLightWhite.Print("Enter your choice (0-5): ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)
		if input == "0" {
			pterm.FgLightWhite.Println("Returning to chat...")
			return nil
		}

		var choice int
		_, err = fmt.Sscanf(input, "%d", &choice)
		if err != nil || choice < 0 || choice > 5 {
			pterm.FgRed.Println("Invalid choice. Please try again.")
			continue
		}

		switch choice {
		case 1:
			if err := a.showAppearanceSettings(); err != nil {
				pterm.FgRed.Printf("Error in appearance settings: %v\n", err)
			}
		case 2:
			if err := a.showDisplaySettings(); err != nil {
				pterm.FgRed.Printf("Error in display settings: %v\n", err)
			}
		case 3:
			if err := a.showChatSettings(); err != nil {
				pterm.FgRed.Printf("Error in chat settings: %v\n", err)
			}
		case 4:
			if err := a.showMCPServerSettings(); err != nil {
				pterm.FgRed.Printf("Error in MCP settings: %v\n", err)
			}
		case 5:
			if err := a.showConfigurationSettings(); err != nil {
				pterm.FgRed.Printf("Error in configuration settings: %v\n", err)
			}
		}
	}
}

// showConfigurationSettings handles API key, base URL, and model configuration
func (a *SimpleAgent) showConfigurationSettings() error {
	// Load current configuration
	config, err := loadConfig()
	if err != nil {
		// If config doesn't exist, create a new one with current values from agent
		config = &Config{
			APIKey:  a.apiKey,
			BaseURL: a.baseURL,
			Model:   a.model,
		}
	}

	pterm.FgLightWhite.Println("\n" + strings.Repeat("═", 50))
	pterm.FgLightCyan.Println("⚙️  CONFIGURATION SETTINGS")
	pterm.FgLightWhite.Println(strings.Repeat("─", 50))

	for {
		pterm.FgLightWhite.Println("\nCurrent configuration:")
		pterm.FgLightWhite.Printf("1. API Key: %s\n", maskAPIKey(config.APIKey))
		pterm.FgLightWhite.Printf("2. Base URL: %s\n", config.BaseURL)
		pterm.FgLightWhite.Printf("3. Model: %s\n", config.Model)
		pterm.FgLightWhite.Println("\n4. Reset all configuration")
		pterm.FgLightWhite.Println("\n0. Back to Settings")

		pterm.FgLightWhite.Print("Enter choice (0-4): ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)
		if input == "0" {
			return nil
		}

		var choice int
		_, err = fmt.Sscanf(input, "%d", &choice)
		if err != nil || choice < 1 || choice > 4 {
			pterm.FgRed.Println("Invalid choice. Please try again.")
			continue
		}

		switch choice {
		case 1:
			// Change API Key
			pterm.FgLightWhite.Print("Enter new API Key: ")
			newAPIKey, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			newAPIKey = strings.TrimSpace(newAPIKey)
			if newAPIKey != "" {
				config.APIKey = newAPIKey
				pterm.FgLightGreen.Printf("✅ API Key updated to: %s\n", maskAPIKey(config.APIKey))
			}
		case 2:
			// Change Base URL
			pterm.FgLightWhite.Print("Enter new Base URL: ")
			newBaseURL, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			newBaseURL = strings.TrimSpace(newBaseURL)
			if newBaseURL != "" {
				config.BaseURL = newBaseURL
				pterm.FgLightGreen.Printf("✅ Base URL updated to: %s\n", config.BaseURL)
			}
		case 3:
			// Change Model
			pterm.FgLightWhite.Print("Enter new Model: ")
			newModel, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			newModel = strings.TrimSpace(newModel)
			if newModel != "" {
				config.Model = newModel
				pterm.FgLightGreen.Printf("✅ Model updated to: %s\n", config.Model)
			}
		case 4:
			// Reset all configuration
			pterm.FgLightYellow.Println("This will delete your saved configuration and require re-entry on next startup.")
			pterm.FgLightWhite.Print("Are you sure? (y/N): ")
			confirmInput, err := reader.ReadString('\n')
			if err != nil {
				return err
			}

			if strings.ToLower(strings.TrimSpace(confirmInput)) == "y" {
				configPath := getConfigPath()
				if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
					pterm.FgRed.Printf("Failed to delete config file: %v\n", err)
				} else {
					pterm.FgLightGreen.Println("✅ Configuration reset. You'll be prompted for new values on next startup.")
				}
			} else {
				pterm.FgLightCyan.Println("Reset cancelled.")
			}
		}

		// Save the updated configuration
		if err := saveConfig(config); err != nil {
			pterm.FgLightYellow.Printf("⚠️  Warning: Could not save configuration: %v\n", err)
		} else {
			pterm.FgLightCyan.Println("Configuration saved successfully.")
		}

		pterm.FgLightWhite.Println("Press Enter to continue...")
		reader.ReadString('\n')
	}
}

// maskAPIKey masks the API key for display (shows first 8 and last 4 characters)
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 12 {
		return "****"
	}
	return apiKey[:8] + "****" + apiKey[len(apiKey)-4:]
}

// showAppearanceSettings handles color customization
func (a *SimpleAgent) showAppearanceSettings() error {
	pterm.FgLightWhite.Println("\n" + strings.Repeat("═", 50))
	pterm.FgLightCyan.Println("🎨 APPEARANCE SETTINGS")
	pterm.FgLightWhite.Println(strings.Repeat("─", 50))

	colors := []struct {
		name  string
		color pterm.Color
	}{
		{"Light Cyan", pterm.FgLightCyan},
		{"Cyan", pterm.FgCyan},
		{"Light Blue", pterm.FgLightBlue},
		{"Blue", pterm.FgBlue},
		{"Light Green", pterm.FgLightGreen},
		{"Green", pterm.FgGreen},
		{"Light Yellow", pterm.FgLightYellow},
		{"Yellow", pterm.FgYellow},
		{"Light Red", pterm.FgLightRed},
		{"Red", pterm.FgRed},
		{"Light Magenta", pterm.FgLightMagenta},
		{"Magenta", pterm.FgMagenta},
		{"Light White", pterm.FgLightWhite},
		{"White", pterm.FgWhite},
		{"Gray", pterm.FgGray},
		{"Black", pterm.FgBlack},
	}

	for {
		pterm.FgLightWhite.Println("\nChoose text color to customize:")
		pterm.FgLightWhite.Printf("1. Assistant (%s): ", a.assistantColor)
		a.getAssistantColorStyle().Println("█████")
		pterm.FgLightWhite.Printf("2. User Input (%s): ", a.userColor)
		a.getUserColorStyle().Println("█████")
		pterm.FgLightWhite.Printf("3. System (%s): ", a.systemColor)
		a.getSystemColorStyle().Println("█████")
		pterm.FgLightWhite.Printf("4. Tools (%s): ", a.toolColor)
		a.getToolColorStyle().Println("█████")
		pterm.FgLightWhite.Printf("5. Errors (%s): ", a.errorColor)
		a.getErrorColorStyle().Println("█████")
		pterm.FgLightWhite.Println("\n0. Back to Settings")

		pterm.FgLightWhite.Print("Enter choice (0-5): ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)
		if input == "0" {
			return nil
		}

		var choice int
		_, err = fmt.Sscanf(input, "%d", &choice)
		if err != nil || choice < 1 || choice > 5 {
			pterm.FgRed.Println("Invalid choice. Please try again.")
			continue
		}

		pterm.FgLightWhite.Println("\nAvailable Colors:")
		for i, color := range colors {
			pterm.FgLightWhite.Printf("%2d. ", i+1)
			color.color.Printf("%s", color.name)
			pterm.FgLightWhite.Println()
		}

		pterm.FgLightWhite.Print("Choose a color (1-16): ")
		colorInput, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		colorInput = strings.TrimSpace(colorInput)
		if colorInput == "" || colorInput == "0" {
			continue
		}

		var colorChoice int
		_, err = fmt.Sscanf(colorInput, "%d", &colorChoice)
		if err != nil || colorChoice < 1 || colorChoice > len(colors) {
			pterm.FgRed.Println("Invalid color choice. Please try again.")
			continue
		}

		selectedColor := colors[colorChoice-1]

		// Map display name to the actual color constant name and update the appropriate field
		switch choice {
		case 1:
			a.assistantColor = a.mapColorName(selectedColor.name)
			pterm.FgLightGreen.Printf("✅ Assistant text color updated to: ")
			selectedColor.color.Println(selectedColor.name)
		case 2:
			a.userColor = a.mapColorName(selectedColor.name)
			pterm.FgLightGreen.Printf("✅ User input color updated to: ")
			selectedColor.color.Println(selectedColor.name)
		case 3:
			a.systemColor = a.mapColorName(selectedColor.name)
			pterm.FgLightGreen.Printf("✅ System message color updated to: ")
			selectedColor.color.Println(selectedColor.name)
		case 4:
			a.toolColor = a.mapColorName(selectedColor.name)
			pterm.FgLightGreen.Printf("✅ Tool output color updated to: ")
			selectedColor.color.Println(selectedColor.name)
		case 5:
			a.errorColor = a.mapColorName(selectedColor.name)
			pterm.FgLightGreen.Printf("✅ Error message color updated to: ")
			selectedColor.color.Println(selectedColor.name)
		}

		pterm.FgLightWhite.Println("Press Enter to continue...")
		reader.ReadString('\n')
	}
}

// showDisplaySettings handles display-related options
func (a *SimpleAgent) showDisplaySettings() error {
	pterm.FgLightWhite.Println("\n" + strings.Repeat("═", 50))
	pterm.FgLightCyan.Println("🖥️  DISPLAY SETTINGS")
	pterm.FgLightWhite.Println(strings.Repeat("─", 50))

	for {
		pterm.FgLightWhite.Println("\nCurrent settings:")
		modeStr := "Normal"
		if a.compactMode {
			modeStr = "Compact"
		}
		pterm.FgLightWhite.Printf("1. Display Mode: %s\n", modeStr)
		pterm.FgLightWhite.Printf("2. Show Timestamps: %t\n", a.showTimestamps)
		pterm.FgLightWhite.Printf("3. Show Hidden Files: %t\n", a.showHidden)
		pterm.FgLightWhite.Println("\n0. Back to Settings")

		pterm.FgLightWhite.Print("Enter choice (0-3): ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)
		if input == "0" {
			return nil
		}

		var choice int
		_, err = fmt.Sscanf(input, "%d", &choice)
		if err != nil || choice < 1 || choice > 3 {
			pterm.FgRed.Println("Invalid choice. Please try again.")
			continue
		}

		switch choice {
		case 1:
			a.compactMode = !a.compactMode
			newMode := "Normal"
			if a.compactMode {
				newMode = "Compact"
			}
			pterm.FgLightGreen.Printf("✅ Display mode changed to: %s\n", newMode)
		case 2:
			a.showTimestamps = !a.showTimestamps
			status := "disabled"
			if a.showTimestamps {
				status = "enabled"
			}
			pterm.FgLightGreen.Printf("✅ Timestamps %s\n", status)
		case 3:
			a.showHidden = !a.showHidden
			status := "disabled"
			if a.showHidden {
				status = "enabled"
			}
			pterm.FgLightGreen.Printf("✅ Hidden files %s\n", status)
		}

		pterm.FgLightWhite.Println("Press Enter to continue...")
		reader.ReadString('\n')
	}
}

// showChatSettings handles chat behavior options
func (a *SimpleAgent) showChatSettings() error {
	pterm.FgLightWhite.Println("\n" + strings.Repeat("═", 50))
	pterm.FgLightCyan.Println("💾 CHAT SETTINGS")
	pterm.FgLightWhite.Println(strings.Repeat("─", 50))

	for {
		pterm.FgLightWhite.Println("\nCurrent settings:")
		pterm.FgLightWhite.Printf("1. Auto-save Chat: %t\n", a.autoSaveChat)
		pterm.FgLightWhite.Println("\n0. Back to Settings")

		pterm.FgLightWhite.Print("Enter choice (0-1): ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)
		if input == "0" {
			return nil
		}

		var choice int
		_, err = fmt.Sscanf(input, "%d", &choice)
		if err != nil || choice != 1 {
			pterm.FgRed.Println("Invalid choice. Please try again.")
			continue
		}

		a.autoSaveChat = !a.autoSaveChat
		status := "disabled"
		if a.autoSaveChat {
			status = "enabled"
		}
		pterm.FgLightGreen.Printf("✅ Auto-save chat %s\n", status)

		pterm.FgLightWhite.Println("Press Enter to continue...")
		reader.ReadString('\n')
	}
}

// showMCPServerSettings handles MCP server configuration
func (a *SimpleAgent) showMCPServerSettings() error {
	pterm.FgLightWhite.Println("\n" + strings.Repeat("═", 50))
	pterm.FgLightCyan.Println("🔌 MCP SERVER SETTINGS")
	pterm.FgLightWhite.Println(strings.Repeat("─", 50))

	for {
		pterm.FgLightWhite.Println("\nConnected MCP Servers:")
		for i, server := range a.servers {
			pterm.FgLightWhite.Printf("%d. %s - %s\n", i+1, server.Name, server.Command)
		}
		pterm.FgLightWhite.Println("\n0. Back to Settings")

		pterm.FgLightWhite.Printf("Enter choice (0-%d): ", len(a.servers))

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)
		if input == "0" {
			return nil
		}

		var choice int
		_, err = fmt.Sscanf(input, "%d", &choice)
		if err != nil || choice < 1 || choice > len(a.servers) {
			pterm.FgRed.Println("Invalid choice. Please try again.")
			continue
		}

		server := a.servers[choice-1]
		pterm.FgLightWhite.Printf("Managing server: %s\n", server.Name)
		pterm.FgLightWhite.Println("1. View server info")
		pterm.FgLightWhite.Println("2. Refresh tools")
		pterm.FgLightWhite.Println("0. Back")

		pterm.FgLightWhite.Print("Enter choice: ")
		actionInput, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		actionInput = strings.TrimSpace(actionInput)
		switch actionInput {
		case "0":
			continue
		case "1":
			pterm.FgLightWhite.Printf("Server: %s\n", server.Name)
			pterm.FgLightWhite.Printf("Command: %s\n", server.Command)
			if len(server.Args) > 0 {
				pterm.FgLightWhite.Printf("Args: %v\n", server.Args)
			}
		case "2":
			if err := a.RefreshTools(); err != nil {
				pterm.FgRed.Printf("Failed to refresh tools: %v\n", err)
			} else {
				pterm.FgLightGreen.Println("✅ Tools refreshed successfully")
			}
		}

		pterm.FgLightWhite.Println("Press Enter to continue...")
		reader.ReadString('\n')
	}
}

// mapColorName converts display name to color constant name
func (a *SimpleAgent) mapColorName(displayName string) string {
	switch displayName {
	case "Light Cyan":
		return "FgLightCyan"
	case "Cyan":
		return "FgCyan"
	case "Light Blue":
		return "FgLightBlue"
	case "Blue":
		return "FgBlue"
	case "Light Green":
		return "FgLightGreen"
	case "Green":
		return "FgGreen"
	case "Light Yellow":
		return "FgLightYellow"
	case "Yellow":
		return "FgYellow"
	case "Light Red":
		return "FgLightRed"
	case "Red":
		return "FgRed"
	case "Light Magenta":
		return "FgLightMagenta"
	case "Magenta":
		return "FgMagenta"
	case "Light White":
		return "FgLightWhite"
	case "White":
		return "FgWhite"
	case "Gray":
		return "FgGray"
	case "Black":
		return "FgBlack"
	default:
		return "FgLightCyan"
	}
}

// showFileBrowser displays an interactive file browser for selecting files
func (a *SimpleAgent) showFileBrowser() (string, error) {
	if a.currentDir == "" {
		// Set current directory to working directory
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		a.currentDir = wd
	}

	for {
		a.getSystemColorStyle().Printf("📁 Current directory: %s\n", a.currentDir)
		a.getSystemColorStyle().Println(strings.Repeat("─", 60))

		// List directories and files
		entries, err := os.ReadDir(a.currentDir)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}

		// Separate directories and files
		var dirs []os.DirEntry
		var files []os.DirEntry

		for _, entry := range entries {
			// Skip hidden files if showHidden is false
			if !a.showHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			if entry.IsDir() {
				dirs = append(dirs, entry)
			} else {
				files = append(files, entry)
			}
		}

		// Display directories first
		if len(dirs) > 0 {
			a.getSystemColorStyle().Println("\n📂 Directories:")
			for i, dir := range dirs {
				a.getSystemColorStyle().Printf("  %2d. 📁 %s\n", i+1, dir.Name())
			}
		}

		// Display files
		if len(files) > 0 {
			a.getSystemColorStyle().Println("\n📄 Files:")
			for i, file := range files {
				fileIndex := len(dirs) + i + 1
				a.getSystemColorStyle().Printf("  %2d. 📄 %s\n", fileIndex, file.Name())
			}
		}

		a.getSystemColorStyle().Println("\n" + strings.Repeat("─", 60))
		hiddenStatus := "OFF"
		if a.showHidden {
			hiddenStatus = "ON"
		}
		a.getSystemColorStyle().Printf("Navigation: [number] Select | [..] Parent dir | [~] Home | [.] Current dir | [/] Root | [q] Cancel (Hidden: %s)\n", hiddenStatus)
		a.getSystemColorStyle().Print("Enter choice: ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		input = strings.TrimSpace(input)
		if input == "q" || input == "Q" {
			return "", nil // Cancelled
		}

		// Handle special navigation commands
		switch input {
		case "..":
			parentDir := filepath.Dir(a.currentDir)
			if parentDir != a.currentDir {
				a.currentDir = parentDir
			}
			continue
		case "~":
			homeDir, _ := os.UserHomeDir()
			a.currentDir = homeDir
			continue
		case ".":
			// Stay in current directory
			continue
		case "/":
			a.currentDir = "/"
			continue
		}

		// Try to parse as number
		var choice int
		if _, err := fmt.Sscanf(input, "%d", &choice); err != nil {
			a.getErrorColorStyle().Println("Invalid choice. Please try again.")
			continue
		}

		// Validate choice
		totalEntries := len(dirs) + len(files)
		if choice < 1 || choice > totalEntries {
			a.getErrorColorStyle().Println("Invalid choice. Please try again.")
			continue
		}

		// Find the selected entry
		var selectedEntry os.DirEntry
		if choice <= len(dirs) {
			selectedEntry = dirs[choice-1]
		} else {
			selectedEntry = files[choice-len(dirs)-1]
		}

		selectedPath := filepath.Join(a.currentDir, selectedEntry.Name())

		// Check if it's a directory
		if selectedEntry.IsDir() {
			a.currentDir = selectedPath
			continue
		}

		// It's a file, return the path
		return selectedPath, nil
	}
}

// handleFileBrowserCommand processes the @ command for file selection
func (a *SimpleAgent) handleFileBrowserCommand() (string, error) {
	a.getSystemColorStyle().Println("🔍 File Browser - Select a file to reference:")
	selectedPath, err := a.showFileBrowser()
	if err != nil {
		return "", err
	}

	if selectedPath == "" {
		a.getSystemColorStyle().Println("File selection cancelled.")
		return "", nil
	}

	// Convert to absolute path for consistency
	absPath, err := filepath.Abs(selectedPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	a.getSystemColorStyle().Printf("✅ Selected file: %s\n", absPath)
	return absPath, nil
}

func (a *SimpleAgent) AddMCPServer(name, command string, args []string) error {
	config := &MCPServerConfig{
		Name:    name,
		Command: command,
		Args:    args,
	}

	transport := &mcp.CommandTransport{Command: exec.Command(command, args...)}
	session, err := a.mcpClient.Connect(a.ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", name, err)
	}

	config.Session = session
	a.servers = append(a.servers, config)

	return nil
}

func (a *SimpleAgent) buildOpenAIToolsFromMCP(ctx context.Context, session *mcp.ClientSession) ([]openai.ChatCompletionToolUnionParam, error) {
	res, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}

	out := make([]openai.ChatCompletionToolUnionParam, 0, len(res.Tools))
	for _, t := range res.Tools {
		var paramsObj map[string]any
		if t.InputSchema != nil {
			raw, err := json.Marshal(t.InputSchema)
			if err != nil {
				return nil, fmt.Errorf("marshal input schema for %s: %w", t.Name, err)
			}
			if err := json.Unmarshal(raw, &paramsObj); err != nil {
				return nil, fmt.Errorf("unmarshal input schema for %s: %w", t.Name, err)
			}
		} else {
			paramsObj = map[string]any{"type": "object", "properties": map[string]any{}}
		}

		// Normalize schema
		if paramsObj == nil {
			paramsObj = map[string]any{}
		}
		if v, ok := paramsObj["type"]; !ok || v != "object" {
			paramsObj["type"] = "object"
		}
		if _, ok := paramsObj["properties"]; !ok {
			paramsObj["properties"] = map[string]any{}
		}
		if props, ok := paramsObj["properties"].(map[string]any); !ok || props == nil {
			paramsObj["properties"] = map[string]any{}
		}

		// Filter out 'uid' parameter
		props := paramsObj["properties"].(map[string]any)
		if _, exists := props["uid"]; exists {
			delete(props, "uid")
			// Also remove 'uid' from required fields if present
			if required, ok := paramsObj["required"].([]any); ok {
				newRequired := make([]any, 0, len(required))
				for _, req := range required {
					if reqStr, ok := req.(string); ok && reqStr != "uid" {
						newRequired = append(newRequired, req)
					}
				}
				paramsObj["required"] = newRequired
			}
		}

		tool := openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  openai.FunctionParameters(paramsObj),
		})
		out = append(out, tool)
	}
	return out, nil
}

func (a *SimpleAgent) GetAllTools() ([]openai.ChatCompletionToolUnionParam, error) {
	var allTools []openai.ChatCompletionToolUnionParam

	for _, server := range a.servers {
		tools, err := a.buildOpenAIToolsFromMCP(a.ctx, server.Session)
		if err != nil {
			log.Printf("Warning: failed to get tools from server %s: %v", server.Name, err)
			continue
		}
		allTools = append(allTools, tools...)
	}

	return allTools, nil
}

// RefreshTools queries all connected MCP servers and caches the available tools.
func (a *SimpleAgent) RefreshTools() error {
	tools, err := a.GetAllTools()
	if err != nil {
		return err
	}
	a.tools = tools
	return nil
}

func (a *SimpleAgent) CallTool(toolName string, arguments map[string]any) (interface{}, error) {
	// Inject uid if this function originally had it (simplified for demo)
	if a.userID != "" {
		if arguments == nil {
			arguments = make(map[string]any)
		}
		arguments["uid"] = a.userID
	}

	// Try each server until we find one that has the tool
	for _, server := range a.servers {
		params := &mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		}

		res, err := server.Session.CallTool(a.ctx, params)
		if err == nil {
			// Tool found and executed successfully
			if len(res.Content) > 0 {
				return res.Content[0], nil
			}
			return "Tool executed successfully", nil
		}
		// If tool not found on this server, try the next one
	}

	return nil, fmt.Errorf("tool %s not found in any connected server", toolName)
}

// ProcessUserInput appends user input, streams a response, executes tools until completion, and updates conversation state.
func (a *SimpleAgent) ProcessUserInput(userInput string) error {
	if strings.TrimSpace(userInput) == "" {
		return nil
	}

	// Append user message to conversation
	a.messages = append(a.messages, openai.UserMessage(userInput))

	// Continue conversation loop until no more tool calls are needed
	for {
		// Show loading spinner
		spinner, _ := pterm.DefaultSpinner.
			WithRemoveWhenDone(true).
			WithShowTimer(false).
			Start("")

		// Create streaming request
		stream := a.openaiClient.Chat.Completions.NewStreaming(a.ctx, openai.ChatCompletionNewParams{
			Messages:          a.messages,
			Model:             openai.ChatModel(a.model),
			Tools:             a.tools,
			ParallelToolCalls: openai.Bool(false),
		})

		// Use ChatCompletionAccumulator to properly handle tool calls
		acc := openai.ChatCompletionAccumulator{}

		for stream.Next() {
			current := stream.Current()
			acc.AddChunk(current)

			// Stop spinner on first content
			spinner.Stop()

			// Stream content to terminal
			if len(current.Choices) > 0 {
				choice := current.Choices[0]
				if choice.Delta.Content != "" {
					a.getAssistantColorStyle().Print(choice.Delta.Content)
				}
			}
		}

		if err := stream.Err(); err != nil {
			spinner.Fail("Error occurred")
			return fmt.Errorf("stream error: %w", err)
		}

		// Check if we have tool calls to process
		if len(acc.Choices) > 0 && len(acc.Choices[0].Message.ToolCalls) > 0 {
			// Add the assistant message with tool calls to conversation
			a.messages = append(a.messages, acc.Choices[0].Message.ToParam())

			// Execute tools and add tool messages
			for _, toolCall := range acc.Choices[0].Message.ToolCalls {
				if toolCall.Function.Name != "" && toolCall.ID != "" {
					spinner, _ := pterm.DefaultSpinner.
						WithRemoveWhenDone(true).
						WithShowTimer(false).
						Start(a.getToolColorStyle().Sprint(fmt.Sprintf("Running %s", toolCall.Function.Name)))

					// Parse arguments
					var args map[string]any
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						spinner.Fail("Failed")
						continue
					}

					// Display tool call details in a dotted box before execution
					a.displayToolCallDetails(toolCall.Function.Name, args)

					// Execute the tool
					result, err := a.CallTool(toolCall.Function.Name, args)
					if err != nil {
						spinner.Fail("Failed")
						a.getErrorColorStyle().Printf("Tool Error: %v\n", err)
						result = fmt.Sprintf("Error: %v", err)
					} else {
						spinner.Success("Done")
					}

					// Display tool result in a dotted box after execution
					a.displayToolResult(toolCall.Function.Name, result, err)

					// Add tool message to conversation
					toolMessage := openai.ToolMessage(fmt.Sprintf("%v", result), toolCall.ID)
					a.messages = append(a.messages, toolMessage)
				}
			}

			continue // Continue the conversation loop
		}

		// No more tool calls; add final assistant message to conversation and finish
		if len(acc.Choices) > 0 {
			a.messages = append(a.messages, acc.Choices[0].Message.ToParam())
		}
		break
	}

	pterm.FgLightWhite.Println("\n" + strings.Repeat("─", 50))
	return nil
}

// ChatLoop starts an interactive REPL for chatting with the agent.
func (a *SimpleAgent) ChatLoop() error {
	reader := bufio.NewReader(os.Stdin)

	_ = pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgBlack)).WithMargin(1).Println("OPEN CODER")
	a.getSystemColorStyle().Println("Type 'exit', 'quit' to end conversation, '/settings' to customize appearance, or '@' to browse files")
	pterm.Println(strings.Repeat("─", 50))

	for {
		pterm.Print("\n" + a.getUserColorStyle().Sprint("You ▸ "))
		text, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		lower := strings.ToLower(text)
		if lower == "exit" || lower == "quit" || lower == "bye" {
			a.getSystemColorStyle().Println("\nGoodbye! 👋")
			return nil
		}
		if lower == "/settings" {
			if err := a.showSettingsMenu(); err != nil {
				a.getErrorColorStyle().Printf("Settings error: %v\n", err)
			}
			continue
		}

		// Handle @ command for file browser
		if strings.HasPrefix(text, "@") {
			selectedPath, err := a.handleFileBrowserCommand()
			if err != nil {
				a.getErrorColorStyle().Printf("File browser error: %v\n", err)
				continue
			}
			if selectedPath != "" {
				// Replace @ with the selected file path
				text = strings.Replace(text, "@", fmt.Sprintf("`%s`", selectedPath), 1)
				a.getSystemColorStyle().Printf("📎 File path inserted: %s\n", selectedPath)
			} else {
				continue // File selection was cancelled
			}
		}

		pterm.Println("\n" + a.getAssistantColorStyle().Sprint("Assistant ▸"))
		if err := a.ProcessUserInput(text); err != nil {
			a.getErrorColorStyle().Printf("Error: %v\n", err)
		}
	}
}

// Close attempts to close all MCP sessions.
func (a *SimpleAgent) Close() {
	for _, s := range a.servers {
		if s.Session != nil {
			// Best-effort close; ignore errors if method missing
			_ = s.Session.Close()
		}
	}
}

// displayToolCallDetails displays tool call arguments in a dotted border box
func (a *SimpleAgent) displayToolCallDetails(toolName string, args map[string]any) {
	a.getToolColorStyle().Println("\n" + strings.Repeat("┌", 60))
	a.getToolColorStyle().Printf("│ 🔧 Tool Call: %s\n", toolName)
	a.getToolColorStyle().Println(strings.Repeat("├", 60))

	if len(args) == 0 {
		a.getSystemColorStyle().Println("│ 📝 Arguments: None")
	} else {
		a.getSystemColorStyle().Println("│ 📝 Arguments:")

		// Pretty print arguments with indentation
		argsJSON, _ := json.MarshalIndent(args, "│   ", "  ")
		argsStr := string(argsJSON)

		// Split into lines and add proper indentation
		lines := strings.Split(argsStr, "\n")
		for _, line := range lines {
			if line != "" {
				a.getSystemColorStyle().Println("│   " + line)
			}
		}
	}

	a.getToolColorStyle().Println(strings.Repeat("└", 60))
}

// displayToolResult displays the result of a tool call in a formatted box
func (a *SimpleAgent) displayToolResult(toolName string, result interface{}, err error) {
	a.getToolColorStyle().Println("\n" + strings.Repeat("┌", 60))
	a.getToolColorStyle().Printf("│ ✅ Tool Result: %s\n", toolName)
	a.getToolColorStyle().Println(strings.Repeat("├", 60))

	if err != nil {
		a.getErrorColorStyle().Printf("│ ❌ Error: %v\n", err)
	} else {
		a.getSystemColorStyle().Println("│ 📄 Output:")

		// Convert result to string and format it nicely
		resultStr := fmt.Sprintf("%v", result)

		// If it's a long result, split it into lines
		if len(resultStr) > 50 {
			lines := strings.Split(resultStr, "\n")
			for i, line := range lines {
				if i < 10 { // Limit to first 10 lines to avoid overwhelming output
					a.getSystemColorStyle().Println("│   " + line)
				} else if i == 10 {
					a.getSystemColorStyle().Println("│   ... (truncated)")
					break
				}
			}
		} else {
			lines := strings.Split(resultStr, "\n")
			for _, line := range lines {
				a.getSystemColorStyle().Println("│   " + line)
			}
		}
	}

	a.getToolColorStyle().Println(strings.Repeat("└", 60))
}

func main() {
	ctx := context.Background()

	// Banner
	letters := putils.LettersFromString("OPEN CODER")
	_ = pterm.DefaultBigText.WithLetters(letters).Render()
	_ = pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgBlack)).WithMargin(1).Println("Open-Coder: A open source CLI coding Agent")

	// Get configuration (environment variables, config file, or prompt user)
	config, err := getConfiguration()
	if err != nil {
		log.Fatalf("Failed to get configuration: %v", err)
	}

	agent := NewSimpleAgent(ctx, config.Model, config.APIKey, config.BaseURL)

	// Store configuration values in agent for settings access
	agent.apiKey = config.APIKey
	agent.baseURL = config.BaseURL

	// Initialize conversation with a helpful default system prompt
	agent.InitConversation("You are a helpful assistant with access to multiple powerful tools. You can use file operations tools to read, write, search, and manage files, as well as terminal command tools to execute any system commands. Always use the appropriate tools when they would help provide accurate information, and think step by step when using tools. Users can type '/settings' to customize the assistant's appearance.")

	// Display welcome message with system color
	agent.getSystemColorStyle().Println("🤖 Assistant initialized successfully!")
	agent.getSystemColorStyle().Printf("💡 Type '/settings' to customize appearance or '@' to browse and reference files\n")

	// Initialize MCP servers quietly (without showing connection details)
	spinner, _ := pterm.DefaultSpinner.Start("Initializing...")

	// Auto-discover and connect to all MCP servers in installation directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to get home directory: %v", err))
		log.Fatalf("Failed to get home directory: %v", err)
	}

	installDir := filepath.Join(homeDir, ".open-coder")
	connectedServers := 0

	// Scan for all *-cli executables in the installation directory
	entries, err := os.ReadDir(installDir)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to scan installation directory: %v", err))
		log.Fatalf("Failed to scan installation directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-cli") {
			continue // Skip directories and non-cli executables
		}

		serverName := strings.TrimSuffix(entry.Name(), "-cli")
		serverPath := filepath.Join(installDir, entry.Name())

		// Check if file is executable
		if info, err := entry.Info(); err == nil {
			if info.Mode()&0111 == 0 {
				continue // Skip non-executable files
			}
		}

		// Try to connect to the MCP server
		if err := agent.AddMCPServer(serverName, serverPath, []string{}); err != nil {
			agent.getErrorColorStyle().Printf("Failed to connect to %s server: %v\n", serverName, err)
			// Don't exit on individual server failures - continue with others
		} else {
			connectedServers++
		}
	}

	if connectedServers == 0 {
		spinner.Fail("No MCP servers found")
		agent.getErrorColorStyle().Println("No MCP servers were found in the installation directory.")
		agent.getErrorColorStyle().Println("Make sure tools are built and installed properly.")
		os.Exit(1)
	}

	// Refresh tools from all connected servers
	if err := agent.RefreshTools(); err != nil {
		spinner.Fail(fmt.Sprintf("Failed to load tools: %v", err))
		agent.getErrorColorStyle().Printf("Failed to load tools: %v\n", err)
		os.Exit(1)
	}

	spinner.Success(fmt.Sprintf("Ready · %d servers", connectedServers))

	// Start interactive chat loop
	if err := agent.ChatLoop(); err != nil {
		log.Fatalf("Chat error: %v", err)
	}

	// Cleanup
	agent.Close()
}
