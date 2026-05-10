# ContextSync CLI - 部署与配置文档

> 版本：v1.0.0
> 最后更新：2025-05-10

---

## 一、环境要求

### 1.1 用户端（运行环境）

| 平台 | 架构 | 最低要求 |
|------|------|----------|
| macOS | x86_64 / arm64 | macOS 10.15+ |
| Linux | x86_64 / arm64 | glibc 2.17+ |
| Windows | x86_64 | Windows 10+ (WSL 或 PowerShell) |

### 1.2 开发环境

| 工具 | 版本 |
|------|------|
| Go | 1.23+ |
| Make | 任意版本 |
| Git | 任意版本 |

---

## 二、安装方式

### 2.1 一键安装脚本

```bash
# macOS / Linux
curl -fsSL https://contextsync.dev/install.sh | bash

# 或指定版本
VERSION=v1.0.0 curl -fsSL https://contextsync.dev/install.sh | bash

# 或指定安装目录
INSTALL_DIR=~/.local/bin curl -fsSL https://contextsync.dev/install.sh | bash
```

### 2.2 Homebrew (macOS)

```bash
brew tap contextsync/tap
brew install contextsync
```

### 2.3 手动下载

从 GitHub Releases 下载对应平台的二进制文件：

```
https://github.com/contextsync/cli/releases

文件命名:
- contextsync-darwin-amd64.tar.gz   (macOS Intel)
- contextsync-darwin-arm64.tar.gz   (macOS Apple Silicon)
- contextsync-linux-amd64.tar.gz    (Linux x86_64)
- contextsync-linux-arm64.tar.gz    (Linux ARM64)
- contextsync-windows-amd64.zip     (Windows)
```

### 2.4 从源码构建

```bash
git clone https://github.com/contextsync/cli.git
cd cli
make build

# 安装到系统路径
sudo make install
```

---

## 三、配置文件

### 3.1 配置文件位置

```
~/.contextsync/
├── config.json          # 主配置文件
├── rules.md             # 规则源文件
├── data/
│   └── memories.db      # SQLite 数据库
├── backups/             # 规则备份
└── logs/                # 日志（可选）
```

### 3.2 配置文件格式 (config.json)

```json
{
  "device_id": "550e8400-e29b-41d4-a716-446655440000",
  "server_url": "https://api.contextsync.dev",
  "rules": {
    "targets": ["claude-code", "cursor", "windsurf"],
    "scope": "project"
  }
}
```

### 3.3 配置项说明

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `device_id` | string | UUID | 设备唯一标识，自动生成 |
| `server_url` | string | `https://api.contextsync.dev` | API 服务器地址 |
| `rules.targets` | []string | 所有工具 | 参与规则同步的工具列表 |
| `rules.scope` | string | `project` | 规则作用域：`project` 或 `global` |

### 3.4 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `CONTEXTSYNC_SERVER_URL` | 覆盖服务器 URL | `https://api-test.contextsync.dev` |
| `CONTEXTSYNC_CONFIG_DIR` | 配置目录 | `~/.config/contextsync` |
| `CONTEXTSYNC_DATA_DIR` | 数据目录 | `~/.local/share/contextsync` |

---

## 四、命令详解

### 4.1 init - 初始化

```bash
contextsync init [flags]

Flags:
  -f, --force    强制重新配置所有工具（忽略限制）
```

执行流程：
1. 创建目录结构
2. 初始化 SQLite 数据库
3. 检测已安装的 AI 工具
4. 配置 MCP（受 Free 限制）
5. 创建默认规则文件

### 4.2 status - 状态查看

```bash
contextsync status
```

输出信息：
- 版本号
- 许可证状态（Free/Pro）
- 试用期剩余天数
- 记忆数量
- 已配置工具数量
- 功能权限

### 4.3 server - MCP Server

```bash
contextsync server
```

启动 MCP Server，通过 stdio 协议通信。通常由 AI 工具自动调用，无需用户手动运行。

### 4.4 sync - 云同步

```bash
contextsync sync
```

执行双向同步：
1. 下载云端记忆
2. 上传本地未同步记忆
3. 标记同步状态
4. 更新最后同步时间

### 4.5 activate - 激活许可证

```bash
contextsync activate <license-key>
```

验证并激活 Pro 许可证。

### 4.6 deactivate - 停用许可证

```bash
contextsync deactivate
```

清除本地许可证信息，保留试用期记录。

### 4.7 upgrade - 升级信息

```bash
contextsync upgrade
```

显示 Pro 版功能对比和订阅方案。

### 4.8 memories - 记忆管理

```bash
contextsync memories list              # 列出所有记忆
contextsync memories show <id>         # 查看记忆详情
contextsync memories delete <id>       # 删除记忆
```

### 4.9 rules - 规则管理

```bash
contextsync rules show                 # 显示当前规则
contextsync rules sync                 # 同步规则到所有工具
```

### 4.10 doctor - 诊断

```bash
contextsync doctor
```

检查：
- Go 版本
- 配置目录
- 数据库
- 规则文件
- 已安装的 AI 工具

---

## 五、发布流程

### 5.1 版本号规范

```
vMAJOR.MINOR.PATCH

示例:
- v1.0.0  正式发布
- v1.0.1  Bug 修复
- v1.1.0  新功能
- v2.0.0  重大更新
```

### 5.2 创建发布

```bash
# 1. 更新版本信息
# 编辑 cmd/contextsync/main.go 中的 version 常量

# 2. 提交更改
git add .
git commit -m "chore: release v1.0.0"

# 3. 创建标签
git tag v1.0.0
git push origin main
git push origin v1.0.0

# 4. GitHub Actions 自动构建并发布
# 查看进度: https://github.com/contextsync/cli/actions
```

### 5.3 构建产物

发布后自动生成以下文件：

```
contextsync-darwin-amd64.tar.gz
contextsync-darwin-arm64.tar.gz
contextsync-linux-amd64.tar.gz
contextsync-linux-arm64.tar.gz
contextsync-windows-amd64.zip
```

### 5.4 手动构建

```bash
# 当前平台
make build

# 所有平台
make build-all

# 创建发布包
make release
```

---

## 六、测试环境配置

### 6.1 切换到测试服务器

```bash
# 方式 1: 环境变量
export CONTEXTSYNC_SERVER_URL=https://api-test.contextsync.dev
contextsync status

# 方式 2: 修改配置文件
contextsync config set server_url https://api-test.contextsync.dev
```

### 6.2 使用测试许可证

测试环境的 License Key 前缀为 `creem_test_`：

```bash
contextsync activate creem_test_xxxxx
```

### 6.3 重置本地数据

```bash
# 清除许可证（保留试用期）
contextsync deactivate

# 完全重置
rm -rf ~/.contextsync
contextsync init
```

---

## 七、故障排除

### 7.1 常见问题

| 问题 | 解决方案 |
|------|----------|
| `command not found` | 确认二进制在 PATH 中 |
| `permission denied` | `chmod +x contextsync` |
| 数据库损坏 | 删除 `~/.contextsync/data/memories.db` 重新 init |
| MCP 连接失败 | 检查工具配置文件中的 MCP 配置 |

### 7.2 日志查看

```bash
# MCP Server 日志
contextsync server 2>&1 | tee ~/contextsync.log

# 数据库查询
sqlite3 ~/.contextsync/data/memories.db ".tables"
sqlite3 ~/.contextsync/data/memories.db "SELECT * FROM license"
```

### 7.3 调试模式

```bash
# 启用详细输出
contextsync -v status
contextsync -v sync
```

---

## 八、卸载

```bash
# 删除二进制
rm /usr/local/bin/contextsync

# 或使用 Homebrew
brew uninstall contextsync

# 可选：删除配置和数据
rm -rf ~/.contextsync
```

---

## 九、Makefile 命令参考

```bash
make build        # 构建当前平台
make build-all    # 构建所有平台
make test         # 运行测试
make test-coverage # 测试覆盖率
make install      # 安装到 /usr/local/bin
make clean        # 清理构建产物
make deps         # 下载依赖
make fmt          # 格式化代码
make lint         # 代码检查
make release      # 创建发布包
make run          # 本地运行
```
