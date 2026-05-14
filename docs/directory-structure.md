# 目录结构说明

## 完整目录树

```
cursor-byok/
├── main.go                          # 应用入口，初始化 Wails 并启动
├── app.go                           # Wails App 结构体，生命周期管理
│
├── internal/                        # 内部包（不可被外部导入）
│   ├── agent/                       # Agent 会话与 LLM 交互核心
│   │   ├── adapter.go               # AdapterTarget 接口定义
│   │   ├── anthropic.go             # Anthropic Claude API 适配器
│   │   ├── background_composer.go   # 后台并行生成器
│   │   ├── background_composer_state.go  # Composer 状态机
│   │   ├── bidi.go                  # BidiAppend 双向追加处理
│   │   ├── bugbot.go                # BugBot 问题自动检测
│   │   ├── frame.go                 # SSE 帧解析与构造
│   │   ├── history.go               # 对话历史记录管理
│   │   ├── history_disk.go          # 历史记录磁盘持久化
│   │   ├── interaction_stream.go    # 交互流处理
│   │   ├── openai.go                # OpenAI API 适配器
│   │   ├── retry.go                 # 请求重试逻辑
│   │   ├── runsse.go                # RunSSE 主循环入口
│   │   ├── session.go               # 会话创建、查找、销毁
│   │   ├── stats.go                 # 用量统计聚合
│   │   ├── tool_exec.go             # 工具执行引擎
│   │   └── tools.go                 # 工具定义与注册
│   │
│   ├── bridge/                      # Wails 前后端桥接层
│   │   ├── proxy_service.go         # ProxyService — Wails 绑定的服务方法
│   │   └── types.go                 # 前后端共享类型定义
│   │
│   ├── certs/                       # CA 证书管理
│   │   └── certs.go                 # 证书生成、安装、信任管理
│   │
│   ├── cursor/                      # Cursor 编辑器集成
│   │   └── cursor.go                # Cursor 进程检测与配置读取
│   │
│   ├── mitm/                        # MITM 中间人代理
│   │   └── proxy.go                 # 代理服务器、路由、请求处理
│   │
│   └── relay/                       # 请求转发与重写
│       ├── gateway.go               # Relay 网关 — 请求转发核心
│       ├── models_template.go       # 模型映射模板
│       └── rewriter.go              # 请求/响应体重写
│
├── frontend/                        # Vue 3 前端应用
│   ├── index.html                   # HTML 入口
│   ├── package.json                 # NPM 依赖
│   ├── vite.config.js               # Vite 构建配置
│   └── src/
│       ├── App.vue                  # 主界面组件
│       ├── style.css                # 全局样式
│       └── main.js                  # Vue 应用入口
│
├── docs/                            # 项目文档
│   ├── architecture.md              # 系统架构设计
│   ├── directory-structure.md       # 本文件 — 目录结构说明
│   ├── coding-standards.md          # 编码规范
│   ├── commit-conventions.md        # 代码提交规范
│   ├── release-process.md           # 发布流程
│   └── responsibility.md            # 职责划分
│
├── build/                           # 构建配置与资源
│   ├── appicon.png                  # 应用图标
│   ├── darwin/                      # macOS 构建配置
│   │   └── Info.plist
│   ├── linux/                       # Linux 构建配置
│   │   └── cursor-byok.desktop
│   └── windows/                     # Windows 构建配置
│       ├── icon.ico                 # Windows 图标
│       └── info.json                # 版本信息
│
├── wails.json                       # Wails 项目配置
├── go.mod                           # Go 模块定义
├── go.sum                           # Go 依赖校验
├── README.md                        # 项目说明
├── CHANGELOG.md                     # 变更日志
├── LICENSE                          # MIT 许可证
└── .editorconfig                    # 编辑器配置
```

## 各目录职责

### `/` 根目录

| 文件 | 职责 |
|------|------|
| `main.go` | 应用入口，创建 Wails 应用实例并启动 |
| `app.go` | `App` 结构体，实现 `WailsInit`/`WailsShutdown` 生命周期 |

### `internal/agent/`

**职责**: Agent 会话管理、LLM 交互、工具执行。

| 文件 | 职责 |
|------|------|
| `session.go` | 会话生命周期：创建、查找、销毁、PlanState |
| `runsse.go` | RunSSE 主循环：接收请求、调用 Provider、返回 SSE 流 |
| `bidi.go` | BidiAppend：向现有会话追加内容 |
| `frame.go` | SSE 帧解析与构造 |
| `interaction_stream.go` | 交互流：管理多轮对话的流式输出 |
| `openai.go` | OpenAI API 适配器：请求构造、响应解析 |
| `anthropic.go` | Anthropic API 适配器：请求构造、响应解析 |
| `retry.go` | 请求重试：指数退避、错误分类 |
| `history.go` | 对话历史：内存中的消息序列 |
| `history_disk.go` | 历史持久化：磁盘读写 |
| `tools.go` | 工具定义：ToolSpec、工具注册表 |
| `tool_exec.go` | 工具执行：调用工具、收集结果 |
| `background_composer.go` | 后台 Composer：并行生成候选代码 |
| `background_composer_state.go` | Composer 状态机 |
| `bugbot.go` | BugBot：自动检测代码问题 |
| `adapter.go` | AdapterTarget 接口：Provider 适配目标 |
| `stats.go` | 用量统计：按 Key/模型/日期聚合 |

### `internal/bridge/`

**职责**: Wails 前后端桥接，将 Go 后端方法暴露给前端。

| 文件 | 职责 |
|------|------|
| `proxy_service.go` | ProxyService：Wails 绑定的服务方法集合 |
| `types.go` | 前后端共享类型：ProxyState, UserConfig, UsageStats 等 |

### `internal/certs/`

**职责**: 本地 CA 证书的生成、安装和管理。

| 文件 | 职责 |
|------|------|
| `certs.go` | CA 生成、证书安装（macOS/Windows/Linux）、信任检查 |

### `internal/cursor/`

**职责**: Cursor 编辑器进程检测和配置读取。

| 文件 | 职责 |
|------|------|
| `cursor.go` | 检测 Cursor 进程、读取 Cursor 配置目录 |

### `internal/mitm/`

**职责**: MITM 代理服务器，拦截和路由 Cursor 的 HTTPS 请求。

| 文件 | 职责 |
|------|------|
| `proxy.go` | 代理服务器生命周期、路由规则、请求处理 |

### `internal/relay/`

**职责**: 请求转发和重写，将 Cursor 格式的请求转换为 LLM Provider 格式。

| 文件 | 职责 |
|------|------|
| `gateway.go` | Relay 网关：HTTP 客户端、请求转发 |
| `rewriter.go` | 请求/响应体重写：模型名映射、字段转换 |
| `models_template.go` | 模型映射模板定义 |

### `frontend/`

**职责**: 基于 Vue 3 的管理界面。

| 文件 | 职责 |
|------|------|
| `src/App.vue` | 主界面：配置、状态、统计 |
| `src/style.css` | 全局样式 |
| `src/main.js` | Vue 应用入口 |

### `docs/`

**职责**: 项目文档，规范和设计记录。

### `build/`

**职责**: 平台特定的构建配置和资源文件。

## 构建产物

| 产物 | 路径 | 说明 |
|------|------|------|
| 可执行文件 | `build/bin/` | Wails 构建输出 |
| 前端资源 | `frontend/dist/` | Vite 构建输出（嵌入到 Go 二进制） |
