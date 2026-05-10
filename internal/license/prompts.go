package license

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// PromptType defines when to show upgrade prompts
type PromptType int

const (
	PromptOnToolLimit PromptType = iota // When hitting tool limit
	PromptOnSaveBlocked                 // When save_memory is blocked
	PromptOnTrialEnding                 // When trial is ending soon
	PromptOnTrialExpired                // When trial has expired
	PromptOnSyncAttempt                 // When trying to use cloud sync
	PromptOnMemoryExpiring              // When memories are about to expire
)

// PromptConfig defines when and what to show
type PromptConfig struct {
	Type       PromptType
	Trigger    string
	Message    string
	Cta        string
	Dismissed  bool
}

// ShouldPromptForToolLimit checks if user needs upgrade for more tools
func (v *Validator) ShouldPromptForToolLimit(detectedTools, configuredTools int) *PromptConfig {
	if v.IsPro() {
		return nil
	}

	maxTools := v.GetMaxTools()
	if configuredTools >= maxTools && detectedTools > maxTools {
		return &PromptConfig{
			Type:    PromptOnToolLimit,
			Trigger: fmt.Sprintf("Detected %d tools, but Free tier only supports %d", detectedTools, maxTools),
			Message: "Upgrade to Pro for unlimited tools",
			Cta:     "contextsync upgrade",
		}
	}
	return nil
}

// ShouldPromptForSave checks if save is blocked
func (v *Validator) ShouldPromptForSave() *PromptConfig {
	if v.IsPro() {
		return nil
	}

	return &PromptConfig{
		Type:    PromptOnSaveBlocked,
		Trigger: "Memory saving is a Pro feature",
		Message: "Free tier: Read-only memory access\nPro tier: Save and manage memories permanently",
		Cta:     "contextsync upgrade",
	}
}

// ShouldPromptForTrial checks if trial is ending or expired
func (v *Validator) ShouldPromptForTrial() *PromptConfig {
	if v.IsPro() {
		return nil
	}

	daysLeft := v.GetTrialDaysLeft()

	if daysLeft <= 0 {
		return &PromptConfig{
			Type:    PromptOnTrialExpired,
			Trigger: "Trial period has ended",
			Message: "Your 14-day trial has expired.\nSome features are now limited.",
			Cta:     "contextsync upgrade",
		}
	}

	if daysLeft <= 3 {
		return &PromptConfig{
			Type:    PromptOnTrialEnding,
			Trigger: fmt.Sprintf("%d days left in trial", daysLeft),
			Message: "Your trial is ending soon.\nUpgrade to keep all features.",
			Cta:     "contextsync upgrade",
		}
	}

	return nil
}

// ShouldPromptForSync checks if sync is blocked
func (v *Validator) ShouldPromptForSync() *PromptConfig {
	if v.IsPro() {
		return nil
	}

	return &PromptConfig{
		Type:    PromptOnSyncAttempt,
		Trigger: "Cloud sync is a Pro feature",
		Message: "Free tier: No cloud sync\nPro tier: Sync memories across all your devices",
		Cta:     "contextsync upgrade",
	}
}

// FormatPrompt formats a prompt for display
func FormatPrompt(p *PromptConfig) string {
	if p == nil {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))
	messageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ctaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

	result := "\n"
	result += titleStyle.Render("  "+p.Trigger) + "\n\n"
	result += messageStyle.Render("  " + p.Message) + "\n\n"
	result += "  Run: " + ctaStyle.Render(p.Cta) + "\n"

	return result
}

// GetUpgradeMessage returns a simple upgrade message
func GetUpgradeMessage(reason string) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	return style.Render(fmt.Sprintf("\n  %s\n  Upgrade: contextsync upgrade\n", reason))
}
