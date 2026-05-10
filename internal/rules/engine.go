package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Engine struct {
	rulesPath string
}

func NewEngine() *Engine {
	// Default rules path
	home, _ := os.UserHomeDir()
	rulesPath := filepath.Join(home, ".contextsync", "rules.md")

	return &Engine{
		rulesPath: rulesPath,
	}
}

// GetRules returns the rules content, optionally filtered by section
func (e *Engine) GetRules(section string) (string, error) {
	content, err := os.ReadFile(e.rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "# Rules\n\nNo rules file found. Run: contextsync init", nil
		}
		return "", err
	}

	if section == "" {
		return string(content), nil
	}

	// Extract specific section
	return e.extractSection(string(content), section), nil
}

func (e *Engine) extractSection(content, section string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inSection := false
	sectionHeader := "## " + section

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if inSection {
				break // End of section
			}
			if strings.EqualFold(line, sectionHeader) || strings.Contains(strings.ToLower(line), strings.ToLower(section)) {
				inSection = true
			}
		}
		if inSection {
			result = append(result, line)
		}
	}

	if len(result) == 0 {
		return fmt.Sprintf("Section '%s' not found.", section)
	}

	return strings.Join(result, "\n")
}

// Compile compiles rules to all target formats
func (e *Engine) Compile() error {
	content, err := os.ReadFile(e.rulesPath)
	if err != nil {
		return err
	}

	home, _ := os.UserHomeDir()

	// Target files
	targets := map[string]string{
		filepath.Join(home, ".claude", "CLAUDE.md"):           string(content),
		filepath.Join(home, ".cursorrules"):                   string(content),
		filepath.Join(home, ".gemini", "GEMINI.md"):            string(content),
		filepath.Join(home, ".codeium", "windsurfrules"):       string(content),
		filepath.Join(home, ".codex", "AGENTS.md"):             string(content),
		filepath.Join(home, ".github", "copilot-instructions.md"): string(content),
	}

	for path, data := range targets {
		// Ensure directory exists
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}

		// Write file
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			continue
		}
	}

	return nil
}
