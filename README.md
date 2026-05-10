# ContextSync CLI

> Cross-tool AI coding context hub - unified rules and memory sync

## Installation

### One-line install (macOS/Linux)

```bash
curl -fsSL https://contextsync.dev/install.sh | sh
```

### Homebrew

```bash
brew install contextsync
```

### Manual download

Download the latest release from [GitHub Releases](https://github.com/contextsync/cli/releases).

## Quick Start

```bash
# Initialize ContextSync
contextsync init

# Check status
contextsync status

# Edit rules
contextsync rules edit

# Start MCP server (usually auto-started by AI tools)
contextsync server
```

## Features

### Free Tier
- ✅ 2 AI tools (Claude Code + Cursor)
- ✅ 14-day memory retention
- ✅ Read-only memory access
- ✅ Local storage

### Pro Tier
- ✅ All 6+ AI tools
- ✅ Permanent memory retention
- ✅ Unlimited memory storage
- ✅ Cloud sync across devices

## Commands

```
contextsync init        Initialize ContextSync
contextsync status      Show current status
contextsync doctor      Run diagnostics
contextsync server      Start MCP server
contextsync upgrade     Upgrade to Pro
contextsync activate    Activate Pro license
contextsync rules       Manage rules
contextsync memories    Manage memories
contextsync config      Manage configuration
```

## Development

### Prerequisites

- Go 1.22+
- SQLite3

### Build

```bash
# Install dependencies
make deps

# Build for current platform
make build

# Build for all platforms
make build-all

# Run tests
make test
```

### Project Structure

```
contextsync-cli/
├── cmd/contextsync/     # CLI entrypoint
├── internal/
│   ├── cli/             # CLI commands
│   ├── mcp/             # MCP Server
│   ├── memory/          # Memory management
│   ├── rules/           # Rules engine
│   ├── license/         # License validation
│   ├── cloud/           # Cloud sync client
│   ├── db/              # SQLite database
│   ├── integrations/    # AI tool integrations
│   └── config/          # Configuration
├── go.mod
└── Makefile
```

## License

MIT
