package cli

import (
	"fmt"
	"os"

	"contextsync/internal/config"
	"contextsync/internal/mcp"
	"github.com/spf13/cobra"
)

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

func runServer() {
	// Initialize
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		return
	}

	if err := initDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		return
	}
	defer closeDatabase()

	// Create MCP server
	server := mcp.NewServer(database)

	// Run server (blocks)
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}
}
