package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"contextsync/internal/config"
	"contextsync/internal/license"
	"contextsync/internal/mcp"
	"contextsync/internal/tools"
	"github.com/spf13/cobra"
)

var toolName string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server.

The MCP server provides tools and resources for AI coding assistants:
- get_memories: Retrieve relevant memories
- save_memory: Save important context (Pro)
- get_rules: Get current coding rules
- list_memories: List all memories

This is typically called automatically by AI tools configured with ContextSync.`,
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

func init() {
	serverCmd.Flags().StringVar(&toolName, "tool", "", "AI tool name for license validation (e.g., \"Claude Code\")")
}

func runServer() {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		return
	}

	// Validate tool name against allowed list
	if toolName != "" {
		if err := validateToolAccess(toolName); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}

	if err := initDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		return
	}
	defer closeDatabase()

	server := mcp.NewServer(database)

	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}
}

func validateToolAccess(name string) error {
	// Local check: read tools.enc
	allowed, err := tools.IsToolAllowed(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read tool config: %v\n", err)
		return nil
	}

	if !allowed {
		return fmt.Errorf("Tool \"%s\" is not configured for this device.\nRun 'contextsync init' to configure your AI tools.", name)
	}

	// License check: enforce tool count limit for Free tier
	licValidator := license.NewValidator("")
	count, err := tools.GetToolCount()
	if err == nil && !licValidator.IsPro() {
		maxTools := licValidator.GetMaxTools()
		if count > maxTools {
			return fmt.Errorf("Free tier supports max %d tools, but %d are configured.\nUpgrade to Pro for unlimited tools: contextsync upgrade", maxTools, count)
		}
	}

	// Cloud validation (best effort, non-blocking)
	go validateToolCloud(name)

	return nil
}

func validateToolCloud(name string) {
	serverURL := config.GetServerURL()
	token := config.GetAuthToken()
	if serverURL == "" || token == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := serverURL + "/api/v1/tools/validate?tool=" + name
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		fmt.Fprintf(os.Stderr, "\nCloud validation: tool \"%s\" is not allowed on this account.\n", name)
	}
}
