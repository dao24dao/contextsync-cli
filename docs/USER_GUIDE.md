# ContextSync 用户使用手册

> 跨工具 AI 编码上下文同步中心 - 统一管理你的规则和记忆

---

## 目录

- [简介](#简介)
- [安装](#安装)
- [快速开始](#快速开始)
- [核心功能](#核心功能)
  - [规则管理](#规则管理)
  - [记忆管理](#记忆管理)
  - [守护进程](#守护进程)
- [订阅计划](#订阅计划)
- [支持的 AI 工具](#支持的-ai-工具)
- [命令参考](#命令参考)
- [常见问题](#常见问题)

---

## 简介

ContextSync 是一个本地优先的 AI 编码上下文同步工具，帮助你在多个 AI 编码助手之间统一管理：

- **规则 (Rules)**: 编码规范、风格指南、架构决策
- **记忆 (Memory)**: 项目上下文、重要决策、错误修复记录

一次配置，所有工具共享相同的上下文。

### 核心特性

| 特性 | Free | Pro |
|------|------|-----|
| 规则同步 | ✅ | ✅ |
| 记忆存储 | ✅ (14天保留) | ✅ (永久保留) |
| 工具数量 | 最多 2 个 | 无限制 |
| 云同步 | ❌ | ✅ |
| 后台守护进程 | ✅ | ✅ |

---

## 安装

### macOS / Linux

```bash
# 使用 Homebrew (推荐)
brew tap contextsync/tap
brew install contextsync

# 或直接下载
curl -sL https://contextsync.yangqing.one/install.sh | bash
```

### Windows

```powershell
# 使用 Scoop
scoop bucket add contextsync https://github.com/contextsync/scoop-bucket
scoop install contextsync

# 或直接下载
# 从 https://github.com/contextsync/contextsync/releases 下载
```

### 验证安装

```bash
contextsync version
```

---

## 快速开始

### 1. 登录账户

```bash
contextsync login
```

按照提示在浏览器中完成登录。

### 2. 初始化

```bash
contextsync init
```

这个命令会：
- 创建 `~/.contextsync/` 目录结构
- 初始化本地数据库
- 检测已安装的 AI 工具
- 配置 MCP 服务器
- 安装并启动后台守护进程

### 3. 编辑规则

```bash
# 查看规则文件路径
contextsync rules edit

# 或直接编辑
vim ~/.contextsync/rules.md
```

### 4. 查看状态

```bash
contextsync status
```

---

## 核心功能

### 规则管理

规则是共享的编码规范和偏好设置，存储在 `~/.contextsync/rules.md` 文件中。

#### 规则文件示例

```markdown
# My Coding Rules

> This file is the single source of truth for all your AI coding tools.

## Language & Framework

- Always use TypeScript, never plain JavaScript
- Use Zod for schema validation
- Prefer functional components in React

## Code Style

- Max line length: 100 characters
- Use named exports, avoid default exports
- Always add JSDoc comments to public functions

## Architecture Decisions

- Use repository pattern for data access
- No direct database calls in controllers
- Prefer composition over inheritance
```

#### 规则命令

```bash
# 查看当前规则
contextsync rules show

# 手动同步规则到所有工具
contextsync rules sync

# 编辑规则（显示文件路径）
contextsync rules edit
```

#### 自动同步

守护进程会自动监听 `rules.md` 的变化，并同步到所有配置的 AI 工具。

---

### 记忆管理

记忆用于存储项目特定的上下文、重要决策和技术笔记。

#### 通过 MCP 保存记忆

AI 工具可以通过 MCP 协议自动保存记忆：

```
AI: 我会记住这个架构决策...
→ 自动调用 save_memory 工具
```

#### 通过 CLI 查看记忆

```bash
# 列出所有记忆
contextsync memories list

# 查看特定记忆
contextsync memories show <memory-id>

# 删除记忆
contextsync memories delete <memory-id>
```

#### 记忆类别

| 类别 | 用途 |
|------|------|
| `decision` | 架构决策、技术选型 |
| `preference` | 编码偏好、风格选择 |
| `todo` | 待办事项、后续任务 |
| `error_fix` | 错误修复记录 |
| `architecture` | 架构说明、系统设计 |
| `other` | 其他信息 |

---

### 守护进程

守护进程 (Daemon) 在后台自动运行，提供以下功能：

- **规则自动同步**: 监听 `rules.md` 变化，自动同步到所有工具
- **记忆云同步**: (Pro) 自动同步记忆到云端
- **开机自启**: 登录时自动启动

#### 守护进程命令

```bash
# 查看守护进程状态
contextsync daemon status

# 启动守护进程
contextsync daemon start

# 停止守护进程
contextsync daemon stop

# 查看日志
contextsync daemon logs

# 手动安装服务
contextsync daemon install

# 卸载服务
contextsync daemon uninstall
```

#### 平台支持

| 平台 | 服务类型 | 自动启动 | 崩溃重启 |
|------|---------|---------|---------|
| macOS | LaunchAgent | ✅ | ✅ |
| Linux | systemd user | ✅ | ✅ |
| Windows | Task Scheduler | ✅ | ❌ |

---

## 订阅计划

### 免费版

- 最多配置 2 个 AI 工具
- 记忆保留 14 天
- 本地规则同步
- 基础功能支持

### Pro 版

**订阅价格:**
- Monthly: $9/月
- Quarterly: $24/季 (节省 11%)
- Yearly: $89/年 (节省 18%)

**Pro 特权:**
- ✅ 无限制配置所有 12+ AI 工具
- ✅ 永久记忆保留
- ✅ 无限记忆存储
- ✅ 跨设备云同步
- ✅ 优先技术支持

### 升级到 Pro

```bash
# 查看升级选项
contextsync upgrade

# 激活许可证
contextsync activate <license-key>
```

---

## 支持的 AI 工具

ContextSync 自动检测并配置以下 AI 编码工具：

### Tier 1: 主流工具

| 工具 | 配置文件 | 状态 |
|------|---------|------|
| Claude Code | `~/.claude/settings.json` | ✅ 完全支持 |
| Cursor | `~/.cursor/mcp.json` | ✅ 完全支持 |
| GitHub Copilot | `~/.github/copilot/mcp.json` | ✅ 完全支持 |
| Windsurf | `~/.codeium/mcp.json` | ✅ 完全支持 |

### Tier 2: 成长中工具

| 工具 | 配置文件 | 状态 |
|------|---------|------|
| Gemini CLI | `~/.gemini/settings.json` | ✅ 完全支持 |
| Codex CLI | `~/.codex/config.json` | ✅ 完全支持 |

### Tier 3: 专业工具

| 工具 | 配置文件 | 状态 |
|------|---------|------|
| Cline | `~/.cline/mcp.json` | ✅ 完全支持 |
| Roo Code | `~/.roo/mcp.json` | ✅ 完全支持 |
| Aider | `~/.aider/mcp.json` | ✅ 完全支持 |
| Continue | `~/.continue/config.json` | ✅ 完全支持 |
| Zed | `~/.zed/settings.json` | ✅ 完全支持 |
| Replit AI | `~/.replit/mcp.json` | ✅ 完全支持 |

---

## 命令参考

### 账户管理

```bash
contextsync login              # 登录账户
contextsync logout             # 登出账户
contextsync status             # 查看当前状态
```

### 初始化与诊断

```bash
contextsync init               # 初始化 ContextSync
contextsync doctor             # 运行诊断检查
contextsync version            # 显示版本信息
contextsync update             # 更新到最新版本
```

### 规则管理

```bash
contextsync rules show         # 显示当前规则
contextsync rules sync         # 同步规则到所有工具
contextsync rules edit         # 显示规则文件路径
```

### 记忆管理

```bash
contextsync memories list      # 列出所有记忆
contextsync memories show <id> # 查看特定记忆
contextsync memories delete <id> # 删除记忆
```

### 守护进程

```bash
contextsync daemon             # 前台运行守护进程
contextsync daemon start       # 启动服务
contextsync daemon stop        # 停止服务
contextsync daemon status      # 查看状态
contextsync daemon logs        # 查看日志
contextsync daemon install     # 安装服务
contextsync daemon uninstall   # 卸载服务
```

### 订阅管理

```bash
contextsync upgrade            # 查看升级选项
contextsync activate <key>     # 激活 Pro 许可证
contextsync deactivate         # 取消激活
contextsync sync               # 手动云同步 (Pro)
```

### MCP 服务器

```bash
contextsync server             # 启动 MCP 服务器
```

---

## 常见问题

### Q: 如何更改规则同步的目标工具？

编辑 `~/.contextsync/rules.md` 后，守护进程会自动同步到所有已配置的工具。要添加新工具，运行 `contextsync init` 重新检测。

### Q: 记忆数据存储在哪里？

所有数据存储在本地：
- 规则: `~/.contextsync/rules.md`
- 数据库: `~/.contextsync/data/memories.db`
- 日志: `~/.contextsync/logs/`

### Q: Free 用户可以使用守护进程吗？

可以！守护进程对 Free 和 Pro 用户都可用。Free 用户可以自动同步规则，Pro 用户额外获得自动云同步记忆功能。

### Q: 如何在多台设备间同步？

Pro 用户可以启用云同步：
1. 在设备 A 运行 `contextsync init`
2. 在设备 B 使用相同账户登录并运行 `contextsync init`
3. 守护进程会自动同步记忆到云端

### Q: 守护进程占用多少资源？

守护进程非常轻量：
- 内存: ~10-20 MB
- CPU: 空闲时几乎为 0
- 仅在文件变化时触发同步

### Q: 如何完全卸载 ContextSync？

```bash
# 停止并卸载守护进程
contextsync daemon uninstall

# 删除配置和数据
rm -rf ~/.contextsync

# 移除可执行文件
# macOS/Linux
sudo rm /usr/local/bin/contextsync

# 或使用包管理器
brew uninstall contextsync
```

---

## 技术支持

- **文档**: https://contextsync.yangqing.one/docs
- **GitHub**: https://github.com/contextsync/contextsync
- **问题反馈**: https://github.com/contextsync/contextsync/issues
- **邮件支持**: support@contextsync.dev

---

*最后更新: 2025年5月14日*
