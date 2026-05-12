# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

ContextSync CLI is a cross-tool AI coding context hub that unifies rules and memory sync across AI coding tools (Claude Code, Cursor, etc.). It provides a local-first memory system with optional cloud sync for Pro users.

## Prerequisites

- Go 1.25+
- SQLite3 (built-in via modernc.org/sqlite)

## Common Commands

```bash
# Development
make run              # Run locally for development
make test             # Run tests with race detection
make test-coverage    # Run tests and generate coverage report
make lint             # Lint code (requires golangci-lint)
make fmt              # Format code

# Building
make build            # Build for current platform
make build-all        # Build for all platforms (darwin/linux/windows, amd64/arm64)
make install          # Build and install to /usr/local/bin

# Release
make release          # Generate release archives (zip files)
make docker           # Build Docker image

# Dependencies
make deps             # Download and tidy dependencies
```

## Architecture

### Entry Point
- `cmd/contextsync/main.go` - CLI entry point; calls `cli.Execute(version, commit, date)`

### Command Layer (`internal/cli/`)
Cobra-based CLI commands. Each file typically maps to one subcommand:
- `root.go` - Root command setup
- `init.go` - Initialize ContextSync
- `status.go` - Show current status
- `doctor.go` - Diagnostics
- `server.go` - Start MCP server
- `upgrade.go` - Upgrade to Pro
- `activate.go` - Activate Pro license
- `login.go` - Cloud login
- `sync.go` - Cloud sync
- `rules.go` - Manage rules
- `memories.go` - Manage memories
- `device.go` - Device management
- `update.go` - Auto-update

### Core Packages
- `internal/mcp/` - Model Context Protocol server implementation
- `internal/memory/` - Memory storage and retrieval (SQLite-backed)
- `internal/rules/` - Rules engine for context injection
- `internal/config/` - Configuration management via Viper
- `internal/db/` - SQLite database setup and migrations
- `internal/license/` - License validation for Pro tier
- `internal/cloud/` - Cloud sync client
- `internal/integrations/` - AI tool integration detection

### Data Flow
1. User runs CLI command (`internal/cli/`)
2. Command interacts with core packages (memory, rules, config)
3. MCP server (`internal/mcp/`) exposes functionality to AI tools
4. Data persisted locally in SQLite (`internal/db/`)
5. Pro users can sync to cloud (`internal/cloud/`)

## Key Patterns

- Configuration stored in `~/.contextsync/` (config.yaml)
- SQLite database at `~/.contextsync/contextsync.db`
- Use `internal/config/config.go` for all config access
- License validation happens via `internal/license/validator.go`
- MCP server uses the official `modelcontextprotocol/go-sdk`
