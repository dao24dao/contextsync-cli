package mcp

import (
	"context"
	"fmt"

	"contextsync/internal/db"
	"contextsync/internal/license"
	"contextsync/internal/memory"
	"contextsync/internal/rules"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	mcpServer *mcp.Server
	memory    *memory.Repository
	rules     *rules.Engine
	license   *license.Validator
}

// NewServer creates a new MCP server
func NewServer(database *db.SQLite) *Server {
	licValidator := license.NewValidator("")
	licValidator.SetDB(database)

	// Pass Pro checker to memory repository
	memRepo := memory.NewRepository(database, memory.WithProChecker(licValidator.IsPro))
	rulesEngine := rules.NewEngine()

	s := &Server{
		memory:  memRepo,
		rules:   rulesEngine,
		license: licValidator,
	}

	// Create MCP server with v1.6.0 API
	s.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "contextsync",
		Version: "1.0.0",
	}, nil)

	// Register tools
	s.registerTools()

	return s
}

// Input types for tools
type getMemoriesArgs struct {
	Query string `json:"query" jsonschema:"The context to find relevant memories for"`
	Limit int    `json:"limit" jsonschema:"Maximum number of memories to return (default: 10)"`
}

type saveMemoryArgs struct {
	Content  string `json:"content" jsonschema:"The information to remember"`
	Category string `json:"category" jsonschema:"Category of the memory (decision, preference, todo, error_fix, architecture, other)"`
}

type getRulesArgs struct {
	Section string `json:"section" jsonschema:"Specific section to retrieve (optional)"`
}

type listMemoriesArgs struct {
	Category string `json:"category" jsonschema:"Filter by category (decision, preference, todo, error_fix, architecture, other)"`
	Limit    int    `json:"limit" jsonschema:"Maximum number to return (default: 20)"`
}

func (s *Server) registerTools() {
	// get_memories - Free feature
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_memories",
		Description: "Retrieve stored memories relevant to the current task",
	}, s.handleGetMemories)

	// save_memory - Pro feature
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "save_memory",
		Description: "Save important context to memory (Pro feature)",
	}, s.handleSaveMemory)

	// get_rules - Free feature
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_rules",
		Description: "Get the current coding rules and preferences",
	}, s.handleGetRules)

	// list_memories - Free feature
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_memories",
		Description: "List stored memories with optional filtering",
	}, s.handleListMemories)
}

func (s *Server) handleGetMemories(ctx context.Context, req *mcp.CallToolRequest, args getMemoriesArgs) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}

	memories := s.memory.Search(args.Query, limit)

	if len(memories) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "No relevant memories found."},
			},
		}, nil, nil
	}

	result := fmt.Sprintf("Found %d relevant memories:\n\n", len(memories))
	for i, m := range memories {
		result += fmt.Sprintf("%d. [%s] %s\n", i+1, m.Category, m.Content)
		if i < len(memories)-1 {
			result += "\n"
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

func (s *Server) handleSaveMemory(ctx context.Context, req *mcp.CallToolRequest, args saveMemoryArgs) (*mcp.CallToolResult, any, error) {
	// Check Pro status
	if !s.license.IsPro() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: `🔒 Memory saving requires ContextSync Pro.

Free tier: Read-only memory access, 14-day retention
Pro tier: Unlimited memory with permanent retention

Run: contextsync upgrade`},
			},
		}, nil, nil
	}

	category := args.Category
	if category == "" {
		category = "other"
	}

	mem, err := s.memory.Create(args.Content, category)
	if err != nil {
		return nil, nil, err
	}

	// TODO: Sync to cloud in background

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Memory saved.\n\nCategory: %s\nID: %s", mem.Category, mem.ID)},
		},
	}, nil, nil
}

func (s *Server) handleGetRules(ctx context.Context, req *mcp.CallToolRequest, args getRulesArgs) (*mcp.CallToolResult, any, error) {
	rules, err := s.rules.GetRules(args.Section)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: rules},
		},
	}, nil, nil
}

func (s *Server) handleListMemories(ctx context.Context, req *mcp.CallToolRequest, args listMemoriesArgs) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 20
	}

	memories := s.memory.List(args.Category, limit)

	if len(memories) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "No memories found."},
			},
		}, nil, nil
	}

	result := fmt.Sprintf("Found %d memories:\n\n", len(memories))
	for i, m := range memories {
		result += fmt.Sprintf("%d. [%s] %s\n", i+1, m.Category, truncate(m.Content, 100))
		if i < len(memories)-1 {
			result += "\n"
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

// Run starts the MCP server using stdio transport
func (s *Server) Run() error {
	return s.mcpServer.Run(context.Background(), &mcp.StdioTransport{})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
