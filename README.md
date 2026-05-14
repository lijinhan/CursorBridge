# CursorBridge

> 本地 MITM 代理 + BYOK 网关，让 Cursor IDE 使用你自己的 API Key 和任意 OpenAI/Anthropic 兼容模型。

## 功能

- **BYOK（Bring Your Own Key）**：配置自己的 OpenAI / Anthropic API Key，无需 Cursor 订阅
- **多模型支持**：同时配置多个模型，在 Cursor 中自由切换
- **MITM 代理**：透明拦截 Cursor 的 agent 请求，转发到你的 BYOK 端点
- **本地 CA**：自动生成并安装本地 CA 证书，支持 HTTPS 拦截
- **Agent 工具链**：支持 Shell、Read、Write、Edit、Glob、Grep、MCP 等工具
- **上下文压缩**：当对话超过模型上下文窗口时自动压缩历史
- **使用统计**：跟踪 Token 用量，按模型/日期统计
- **系统托盘**：最小化到系统托盘，后台运行

## 快速开始

### 前置要求

- Go 1.22+
- Node.js 18+（前端构建）
- [Wails v3](https://wails.io/) CLI

### 构建

```bash
# 安装 Wails CLI
go install github.com/nicedoc/wails/v3/cmd/wails3@latest

# 构建
wails3 build

# 或开发模式
wails3 dev
```

### 配置

1. 启动 CursorBridge
2. 在「模型」标签页添加你的 API Key 和模型
3. 点击「启动服务」
4. 在 Cursor IDE 中设置 HTTP 代理为 `http://127.0.0.1:9090`

## 架构

```
Cursor IDE → MITM Proxy → CursorBridge Agent → OpenAI/Anthropic API
                ↓
         请求拦截 & 重写
         SSE 流式转发
         工具调用执行
```

### 核心模块

| 模块 | 路径 | 职责 |
|------|------|------|
| MITM 代理 | `internal/mitm/` | 拦截 Cursor 的 HTTPS 请求 |
| Bridge | `internal/bridge/` | 请求路由、配置管理 |
| Agent | `internal/agent/` | SSE 流处理、工具执行、会话管理 |
| Relay | `internal/relay/` | OpenAI/Anthropic API 适配 |
| 证书 | `internal/certs/` | 本地 CA 生成与安装 |
| Proto Codec | `internal/protocodec/` | Cursor 协议编解码 |
| 前端 | `frontend/` | Wails v3 + Vue 3 管理界面 |

### Agent 子模块

| 文件 | 职责 |
|------|------|
| `runsse.go` | HandleRunSSE 编排器 + 类型定义 |
| `runsse_setup.go` | 会话初始化 + keepalive |
| `runsse_loop.go` | 主流式循环 |
| `runsse_finalize.go` | 持久化 + 清理 |
| `toolbuilder.go` | 工具构建器注册表 |
| `toolbuilder_mcp.go` | MCP 工具构建 |
| `toolbuilder_fs.go` | 文件系统工具构建 |
| `toolbuilder_exec.go` | Shell/Glob/Grep 等工具构建 |
| `deps.go` | 全局可变状态 (AgentDeps) |
| `compaction.go` | 上下文压缩 |

## 配置文件

配置存储在 `~/.cursorbridge/config.json`（Windows: `%APPDATA%\cursorbridge\config.json`）。

### 模型适配器配置

```json
{
  "baseURL": "",
  "modelAdapters": [
    {
      "displayName": "GPT-4o",
      "type": "openai",
      "baseURL": "https://api.openai.com/v1",
      "apiKey": "sk-...",
      "modelID": "gpt-4o"
    },
    {
      "displayName": "Claude Sonnet",
      "type": "anthropic",
      "baseURL": "https://api.anthropic.com",
      "apiKey": "sk-ant-...",
      "modelID": "claude-sonnet-4-5"
    }
  ],
  "activeModelID": "gpt-4o"
}
```

### 高级选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `maxLoopRounds` | Agent 工具循环最大轮次 | 0（不限制） |
| `maxTurnDurationMin` | Agent 会话最大时长（分钟） | 0（不限制） |
| `maxOutputTokens` | 每次响应最大输出 token | 0（提供商默认） |
| `thinkingBudget` | Anthropic 扩展思考 token 预算 | 0 |
| `retryCount` | 上游请求失败重试次数 | 0 |
| `timeout` | 单次请求超时（毫秒） | 0（5分钟） |

## 开发

```bash
# 运行测试
go test ./...

# 构建
go build ./...

# 前端开发
cd frontend && npm run dev
```

## 致谢

本项目在开发过程中参考了以下开源项目的思路和实现：

- **[Yimikami/cursorbridge](https://github.com/Yimikami/cursorbridge)** — 原始项目，提供了核心的 BYOK 代理思路和实现基础
- **[burpheart/cursor-tap](https://github.com/burpheart/cursor-tap)** — MITM 代理和证书管理的参考实现
- **[leookun/cursorbridge-server](https://github.com/leookun/cursorbridge-server)** — 服务端架构的参考设计

## 许可证

MIT
