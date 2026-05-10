# ContextSync CLI - 功能设计文档

> 版本：v1.0.0
> 最后更新：2025-05-10

---

## 一、项目概述

ContextSync CLI 是一个本地运行的命令行工具，提供 MCP Server、多工具配置、记忆管理和规则同步功能。采用 Go 语言编写，编译为单一二进制文件，支持 macOS、Linux 和 Windows。

### 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.23+ |
| CLI 框架 | Cobra |
| 配置管理 | Viper |
| 数据库 | SQLite (modernc.org/sqlite) + FTS5 |
| MCP 协议 | github.com/modelcontextprotocol/go-sdk |
| UI 样式 | Lipgloss |

---

## 二、已实现功能清单

### 2.1 CLI 命令

| 命令 | 功能 | Free | Pro |
|------|------|------|-----|
| `init` | 初始化配置和工具 | ✅ | ✅ |
| `status` | 查看状态和许可证 | ✅ | ✅ |
| `doctor` | 诊断检查 | ✅ | ✅ |
| `version` | 版本信息 | ✅ | ✅ |
| `upgrade` | 升级到 Pro | ✅ | ✅ |
| `activate <key>` | 激活许可证 | ✅ | ✅ |
| `deactivate` | 停用许可证 | ✅ | ✅ |
| `server` | 启动 MCP Server | ✅ | ✅ |
| `memories list` | 列出记忆 | ✅ | ✅ |
| `memories show <id>` | 查看记忆 | ✅ | ✅ |
| `memories delete <id>` | 删除记忆 | ✅ | ✅ |
| `rules show` | 显示规则 | ✅ | ✅ |
| `rules sync` | 同步规则 | ✅ | ✅ |
| `sync` | 云同步 | ❌ | ✅ |

### 2.2 MCP Server 功能接口（4个）

| 接口 | 功能 | Free | Pro |
|------|------|------|-----|
| `get_memories` | 搜索相关记忆 | ✅ | ✅ |
| `list_memories` | 列出记忆 | ✅ | ✅ |
| `get_rules` | 获取规则 | ✅ | ✅ |
| `save_memory` | 保存记忆 | ❌ | ✅ |

### 2.3 支持的 AI 编码工具（12+）

```
Tier 1（主流工具）:
├── Claude Code
├── Cursor
├── Windsurf
└── GitHub Copilot

Tier 2（增长中）:
├── Gemini CLI
└── Codex CLI

Tier 3（专业工具）:
├── Cline
├── Roo Code
├── Aider
├── Continue
├── Zed
└── Replit AI
```

### 2.4 本地存储

| 组件 | 路径 | 说明 |
|------|------|------|
| 配置目录 | `~/.contextsync/` | 主目录 |
| 数据库 | `~/.contextsync/data/memories.db` | SQLite + FTS5 |
| 规则文件 | `~/.contextsync/rules.md` | 统一规则源 |
| 配置文件 | `~/.contextsync/config.json` | 用户配置 |
| 备份目录 | `~/.contextsync/backups/` | 规则备份 |

---

## 三、Free 与 Pro 功能对比

### 3.1 功能限制

| 功能 | Free | Pro |
|------|------|-----|
| **工具数量** | 2 个 | 无限 |
| **记忆保存** | ❌ 只读 | ✅ |
| **记忆过期** | 14 天 | 永久 |
| **云同步** | ❌ | ✅ |
| **试用期** | 14 天 | - |

### 3.2 订阅方案

| 方案 | 价格 | 节省 |
|------|------|------|
| Monthly | $9/月 | - |
| Quarterly | $24/季 | 11% |
| Yearly | $72/年 | 33% |

---

## 四、升级 Pro 提示触发点

### 4.1 触发场景

| 触发点 | 场景 | 提示信息 |
|--------|------|----------|
| **工具限制** | 检测到超过 2 个工具 | "Detected X tools, Free tier only supports 2" |
| **保存阻止** | 调用 `save_memory` | "Memory saving requires ContextSync Pro" |
| **试用期警告** | 剩余 ≤3 天 | "Your trial is ending soon" |
| **试用期过期** | 试用期结束 | "Trial expired. Some features are limited" |
| **云同步尝试** | 运行 `sync` 命令 | "Cloud sync is a Pro feature" |
| **记忆即将过期** | 有记忆 3 天内过期 | status 命令显示警告 |

### 4.2 提示展示位置

```
命令行提示:
├── init 命令输出
├── status 命令输出
├── sync 命令（非 Pro）
├── MCP save_memory 响应
└── memories list（显示过期警告）
```

---

## 五、数据模型

### 5.1 记忆表 (memories)

```sql
CREATE TABLE memories (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    category TEXT DEFAULT 'other',    -- decision/preference/todo/error_fix/architecture/other
    source TEXT DEFAULT 'manual',     -- claude-code/cursor/gemini-cli/manual
    project TEXT,
    tags TEXT DEFAULT '[]',           -- JSON array
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    device_id TEXT NOT NULL,
    synced INTEGER DEFAULT 0,         -- 0=未同步, 1=已同步
    expires_at TEXT,                  -- NULL=永久, 有值=过期时间
    dedup_hash TEXT                   -- SHA256 去重
);
```

### 5.2 许可证表 (license)

```sql
CREATE TABLE license (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    tier TEXT DEFAULT 'free',         -- free/pro
    license_key TEXT,
    subscription_type TEXT,           -- monthly/quarterly/yearly
    valid_until TEXT,
    first_seen_at TEXT,              -- 试用期起始时间
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

### 5.3 已配置工具表 (configured_tools)

```sql
CREATE TABLE configured_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tool_name TEXT NOT NULL UNIQUE,
    config_path TEXT NOT NULL,
    configured_at TEXT NOT NULL
);
```

### 5.4 配置表 (config)

```sql
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

---

## 六、核心模块架构

```
internal/
├── cli/                    # CLI 命令实现
│   ├── root.go            # 根命令和全局变量
│   ├── init.go            # 初始化命令
│   ├── status.go          # 状态命令
│   ├── doctor.go          # 诊断命令
│   ├── server.go          # MCP Server 命令
│   ├── sync.go            # 云同步命令
│   ├── activate.go        # 激活/停用命令
│   ├── upgrade.go         # 升级命令
│   ├── memories.go        # 记忆管理命令
│   └── rules.go           # 规则管理命令
│
├── mcp/                    # MCP Server
│   └── server.go          # 工具定义和处理
│
├── memory/                 # 记忆存储
│   └── repository.go      # CRUD 操作、FTS 搜索、过期管理
│
├── license/                # 许可证管理
│   ├── validator.go       # 验证逻辑、缓存
│   └── prompts.go         # 升级提示模板
│
├── cloud/                  # 云同步客户端
│   └── client.go          # Upload/Download/Merge
│
├── integrations/           # 工具集成
│   └── detector.go        # 工具检测和配置
│
├── rules/                  # 规则引擎
│   └── engine.go          # 规则解析和编译
│
├── db/                     # 数据库
│   └── sqlite.go          # 连接和迁移
│
└── config/                 # 配置管理
    └── config.go          # 路径、设备ID、服务器URL
```

---

## 七、与 PRD 对比

### 7.1 已实现功能 ✅

| PRD 功能 | 实现状态 | 备注 |
|----------|----------|------|
| MCP Server | ✅ 完成 | 支持 4 个工具 |
| 本地 SQLite 存储 | ✅ 完成 | 含 FTS5 全文搜索 |
| 一键安装 | ✅ 完成 | 脚本 + Homebrew |
| 多工具配置 | ✅ 完成 | 12+ 工具检测 |
| 规则文件编译 | ✅ 完成 | 自动同步到工具 |
| 记忆捕获 | ⚠️ 部分 | 通过 MCP 主动保存，Hooks 未实现 |
| 云同步 | ✅ 完成 | CLI + Server 端 |

### 7.2 未实现功能 ❌

| PRD 功能 | 状态 | 计划 |
|----------|------|------|
| 被动记忆捕获 (Hooks) | ❌ | 需要用户配置 Claude Code Hooks |
| 规则文件监听 | ❌ | 手动 `rules sync` |
| 记忆导出 | ❌ | 未来版本 |
| Web Dashboard | ❌ | 未来版本 |
| 团队协作 | ❌ | 未来版本 |

### 7.3 差异说明

| PRD 规格 | 实际实现 | 原因 |
|----------|----------|------|
| Free 5 条记忆 | Free 2 工具 + 14 天过期 | 调整为更合理的限制 |
| 月/年订阅 | 月/季/年订阅 | 增加季度选项 |
| 存储容量限制 | 时间限制 | 时间限制更易理解 |

---

## 八、安全考虑

### 8.1 本地存储安全

- SQLite 数据库存储在用户目录，权限 0600
- 无敏感信息明文存储
- License Key 存储在本地数据库

### 8.2 网络通信

- 仅与配置的服务器通信（默认 `api.contextsync.dev`）
- HTTPS 加密传输
- License 验证失败时不阻止核心功能

### 8.3 Go 二进制优势

- 编译后难以逆向（相比 TypeScript）
- 核心验证逻辑在服务端
- 无法通过修改本地代码绕过限制
