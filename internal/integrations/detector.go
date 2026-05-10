package integrations

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Tool struct {
	Name       string
	ConfigPath string
	Configured bool
	Index      int // Order of detection
}

type Detector struct {
	homeDir string
}

func NewDetector() *Detector {
	home, _ := os.UserHomeDir()
	return &Detector{homeDir: home}
}

// Tool definitions with priority order
var toolDefinitions = []struct {
	name       string
	configPath string
	configDir  string
}{
	// Tier 1: Most popular AI coding assistants
	{"Claude Code", "settings.json", ".claude"},
	{"Cursor", "mcp.json", ".cursor"},
	{"Windsurf", "mcp.json", ".codeium"},
	{"GitHub Copilot", "mcp.json", ".github/copilot"},

	// Tier 2: Growing tools
	{"Gemini CLI", "settings.json", ".gemini"},
	{"Codex CLI", "config.json", ".codex"},

	// Tier 3: Specialized tools
	{"Cline", "mcp.json", ".cline"},
	{"Roo Code", "mcp.json", ".roo"},
	{"Aider", "mcp.json", ".aider"},
	{"Continue", "config.json", ".continue"},
	{"Zed", "settings.json", ".zed"},
	{"Replit AI", "mcp.json", ".replit"},
}

// DetectAll detects all installed AI tools
func (d *Detector) DetectAll() []*Tool {
	var tools []*Tool

	for i, def := range toolDefinitions {
		configDir := filepath.Join(d.homeDir, def.configDir)
		if d.exists(configDir) {
			tools = append(tools, &Tool{
				Name:       def.name,
				ConfigPath: filepath.Join(configDir, def.configPath),
				Index:      i,
			})
		}
	}

	return tools
}

// Configure configures MCP for a tool
func (t *Tool) Configure() error {
	// Create config directory if needed
	dir := filepath.Dir(t.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Check if config already exists
	existingConfig, _ := os.ReadFile(t.ConfigPath)

	// Create MCP config based on whether config exists
	var config []byte
	if len(existingConfig) > 0 {
		// Merge with existing config
		config = mergeMCPConfig(existingConfig)
	} else {
		// Create new config
		config = newMCPConfig()
	}

	return os.WriteFile(t.ConfigPath, config, 0644)
}

func newMCPConfig() []byte {
	return []byte(`{
  "mcpServers": {
    "contextsync": {
      "command": "contextsync",
      "args": ["server"]
    }
  }
}`)
}

func mergeMCPConfig(existing []byte) []byte {
	var config map[string]interface{}
	if err := json.Unmarshal(existing, &config); err != nil {
		// If parsing fails, create new config
		return newMCPConfig()
	}

	// Ensure mcpServers exists
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	// Add contextsync server
	mcpServers["contextsync"] = map[string]interface{}{
		"command": "contextsync",
		"args":    []string{"server"},
	}

	result, _ := json.MarshalIndent(config, "", "  ")
	return result
}

func (d *Detector) exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetAllToolNames returns all supported tool names (for display)
func GetAllToolNames() []string {
	names := make([]string, len(toolDefinitions))
	for i, def := range toolDefinitions {
		names[i] = def.name
	}
	return names
}

// GetToolCount returns total number of supported tools
func GetToolCount() int {
	return len(toolDefinitions)
}
