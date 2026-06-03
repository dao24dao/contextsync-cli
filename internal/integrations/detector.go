package integrations

import (
	"encoding/json"
	"fmt"
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
	{"Aider", "mcp.json", ".aider"},
	{"Continue", "config.json", ".continue"},
	{"Zed", "settings.json", ".zed"},
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

// Configure configures MCP for a tool with --tool parameter
func (t *Tool) Configure() error {
	dir := filepath.Dir(t.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	existingConfig, _ := os.ReadFile(t.ConfigPath)

	var config []byte
	if len(existingConfig) > 0 {
		config = mergeMCPConfig(existingConfig, t.Name)
	} else {
		config = newMCPConfig(t.Name)
	}

	return os.WriteFile(t.ConfigPath, config, 0644)
}

func newMCPConfig(toolName string) []byte {
	return []byte(fmt.Sprintf(`{
  "mcpServers": {
    "contextsync": {
      "command": "contextsync",
      "args": ["server", "--tool", %q]
    }
  }
}`, toolName))
}

func mergeMCPConfig(existing []byte, toolName string) []byte {
	var config map[string]interface{}
	if err := json.Unmarshal(existing, &config); err != nil {
		return newMCPConfig(toolName)
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	mcpServers["contextsync"] = map[string]interface{}{
		"command": "contextsync",
		"args":    []string{"server", "--tool", toolName},
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
