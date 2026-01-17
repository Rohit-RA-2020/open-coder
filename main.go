package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"open-coder/ui"
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
		homeDir = "~"
	}
	return filepath.Join(homeDir, ".open-coder", "config")
}

// loadConfig reads configuration from file
func loadConfig() (*Config, error) {
	configPath := getConfigPath()

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
	fmt.Println()
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#58a6ff")).
		MarginBottom(1)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7d8590"))

	fmt.Println(titleStyle.Render("🔧 First-time Setup"))
	fmt.Println(subtitleStyle.Render("Please provide your OpenAI configuration:"))
	fmt.Println(subtitleStyle.Render("This will be saved to ~/.open-coder/config for future use."))
	fmt.Println()

	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e6edf3"))

	// Prompt for API key
	if apiKey == "" {
		fmt.Print(promptStyle.Render("API Key: "))
		fmt.Scanln(&apiKey)
		apiKey = strings.TrimSpace(apiKey)
	}

	// Prompt for base URL
	if baseURL == "" {
		fmt.Print(promptStyle.Render("Base URL: "))
		fmt.Scanln(&baseURL)
		baseURL = strings.TrimSpace(baseURL)
	}

	// Prompt for model
	if model == "" {
		fmt.Print(promptStyle.Render("Model: "))
		fmt.Scanln(&model)
		model = strings.TrimSpace(model)
	}

	config = &Config{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
	}

	// Save configuration
	if err := saveConfig(config); err != nil {
		fmt.Printf("⚠️  Warning: Could not save configuration: %v\n", err)
	} else {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3fb950"))
		fmt.Println(successStyle.Render("✅ Configuration saved!"))
	}
	fmt.Println()

	return config, nil
}

// appModel wraps the UI model with agent
type appModel struct {
	ui.Model
	agent *ui.Agent
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		m.Model.Init(),
		m.initializeAgent(),
	)
}

func (m appModel) initializeAgent() tea.Cmd {
	return func() tea.Msg {
		// Discover and connect to MCP servers
		serverCount, err := m.agent.DiscoverAndConnectServers()
		if err != nil || serverCount == 0 {
			return ui.MCPErrorMsg{Err: fmt.Errorf("no MCP servers found")}
		}

		// Refresh tools
		if err := m.agent.RefreshTools(); err != nil {
			return ui.MCPErrorMsg{Err: err}
		}

		return ui.MCPConnectedMsg{
			ServerCount: m.agent.GetServerCount(),
			ToolCount:   m.agent.GetToolCount(),
		}
	}
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle agent initialization messages
	switch msg := msg.(type) {
	case ui.MCPErrorMsg:
		// Add error to messages and continue
		return m, nil
	case ui.MCPConnectedMsg:
		// Agent is ready, send init complete
		var cmds []tea.Cmd
		newModel, cmd := m.Model.Update(msg)
		m.Model = newModel.(ui.Model)
		cmds = append(cmds, cmd)
		cmds = append(cmds, func() tea.Msg { return ui.InitCompleteMsg{} })
		return m, tea.Batch(cmds...)
	}

	// Delegate to UI model
	newModel, cmd := m.Model.Update(msg)
	m.Model = newModel.(ui.Model)
	return m, cmd
}

func (m appModel) View() string {
	return m.Model.View()
}

func main() {
	ctx := context.Background()

	// Get configuration
	config, err := getConfiguration()
	if err != nil {
		log.Fatalf("Failed to get configuration: %v", err)
	}

	// Create agent
	agent := ui.NewAgent(ctx, config.Model, config.APIKey, config.BaseURL)
	defer agent.Close()

	// Initialize conversation
	agent.InitConversation(`You are a helpful AI coding assistant with access to powerful tools. You can:
- Read, write, search, and manage files
- Execute terminal commands
- Browse and navigate the codebase

Always use tools when they would help provide accurate information. Think step by step when using tools.
Be concise but thorough in your responses.`)

	// Create UI model
	uiModel := ui.New(agent)

	// Create app model that wraps UI and agent
	model := appModel{
		Model: uiModel,
		agent: agent,
	}

	// Create the Bubble Tea program
	// Setup logging to file
	f, err := tea.LogToFile("open-coder.log", "debug")
	if err != nil {
		log.Fatalf("fatal: could not open log file: %v", err)
	}
	defer f.Close()

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer (full screen)
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Set program reference in agent for sending messages
	agent.SetProgram(p)

	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
