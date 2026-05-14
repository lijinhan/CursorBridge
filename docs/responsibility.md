# 职责划分

## 模块职责边界

### `internal/agent/` — Agent 会话与 LLM 交互

**核心职责**:
- 管理 Agent 会话的完整生命周期（创建、查找、销毁）
- 调用 LLM Provider（OpenAI / Anthropic）获取响应
- 处理 RunSSE 和 BidiAppend 两种交互模式
- 维护对话历史记录
- 执行工具调用
- 管理后台 Composer 和 BugBot

**不负责**:
- ❌ HTTP 请求拦截和路由（属于 `mitm`）
- ❌ 请求/响应格式重写（属于 `relay`）
- ❌ 前端 UI 状态管理（属于 `bridge`）
- ❌ 证书管理（属于 `certs`）

**对外接口**:
- `HandleRunSSE()` — 处理新的 SSE 对话请求
- `HandleBidiAppend()` — 处理双向追加请求
- `ComputeUsageStats()` — 计算用量统计

---

### `internal/bridge/` — Wails 前后端桥接

**核心职责**:
- 将 Go 后端方法暴露为 Wails 前端可调用的服务
- 管理应用状态（ProxyState）
- 读写用户配置（UserConfig）
- 管理模型适配器配置（ModelAdapterConfig）
- 触发证书安装和 Cursor 设置

**不负责**:
- ❌ 直接处理 LLM 请求（属于 `agent`）
- ❌ 请求转发（属于 `relay`）
- ❌ 代理服务器运行（属于 `mitm`）

**对外接口**:
- `ProxyService` — Wails 绑定的服务方法集合
- 所有前端可调用的方法通过 `ProxyService` 暴露

---

### `internal/mitm/` — MITM 中间人代理

**核心职责**:
- 运行本地 HTTPS 代理服务器
- 拦截 Cursor 到 api2.cursor.sh 的请求
- 路由请求到对应的处理器
- 处理 TLS 证书（使用 `certs` 包生成的 CA）

**不负责**:
- ❌ 业务逻辑处理（属于 `agent`）
- ❌ 请求格式转换（属于 `relay`）
- ❌ 用户配置管理（属于 `bridge`）

**对外接口**:
- `New()` — 创建代理实例
- `Start()` / `Stop()` — 启动/停止代理

---

### `internal/relay/` — 请求转发与重写

**核心职责**:
- 将 Cursor 格式的请求转发到 LLM Provider
- 重写请求体（模型名映射、字段转换）
- 重写响应体（适配 Cursor 期望的格式）
- 管理模型映射模板

**不负责**:
- ❌ 会话管理（属于 `agent`）
- ❌ 请求拦截（属于 `mitm`）
- ❌ 用户配置（属于 `bridge`）

**对外接口**:
- `Gateway` — 请求转发网关
- `Rewriter` — 请求/响应重写器

---

### `internal/certs/` — CA 证书管理

**核心职责**:
- 生成本地 CA 根证书
- 为 api2.cursor.sh 签发 TLS 证书
- 安装 CA 到系统信任存储（macOS/Windows/Linux）
- 检查 CA 安装状态

**不负责**:
- ❌ 代理服务器运行（属于 `mitm`）
- ❌ 任何业务逻辑

**对外接口**:
- `EnsureCA()` — 确保 CA 存在（不存在则创建）
- `InstallCA()` — 安装 CA 到系统
- `IsCATrusted()` — 检查 CA 是否被信任

---

### `internal/cursor/` — Cursor 编辑器集成

**核心职责**:
- 检测 Cursor 编辑器进程
- 读取 Cursor 配置目录
- 设置 Cursor 代理配置

**不负责**:
- ❌ 代理服务器运行（属于 `mitm`）
- ❌ 证书管理（属于 `certs`）

**对外接口**:
- `DetectCursor()` — 检测 Cursor 进程
- `SetProxyConfig()` — 设置代理配置

---

### `frontend/` — Vue 3 管理界面

**核心职责**:
- 显示代理状态
- 配置 API Key 和模型适配
- 显示用量统计
- 触发证书安装

**不负责**:
- ❌ 任何业务逻辑（所有逻辑在后端）
- ❌ 直接调用 LLM API

---

## 依赖方向规则

### 允许的依赖方向

```
main/app → bridge → agent → relay
                 ↘ certs
                 ↘ cursor
main/app → mitm → agent → relay
               ↘ certs
```

### 依赖层级

| 层级 | 包 | 可依赖 |
|------|-----|--------|
| 入口层 | `main`, `app` | 所有包 |
| 服务层 | `bridge`, `mitm` | 核心层、基础层 |
| 核心层 | `agent`, `relay` | 标准库、第三方库 |
| 基础层 | `certs`, `cursor` | 标准库、第三方库 |

### 禁止的依赖

1. **禁止循环依赖**: A → B → A
2. **禁止反向依赖**: 核心层不能依赖服务层，基础层不能依赖核心层
3. **禁止跨层依赖**: 入口层以外的包不能依赖入口层
4. **同级包谨慎依赖**: `agent` 和 `relay` 之间保持单向依赖

## 新增代码归属判断

### 判断流程

```
新增功能
  │
  ├─ 涉及 HTTPS 请求拦截？ → mitm
  ├─ 涉及 LLM API 调用？ → agent
  ├─ 涉及请求格式转换？ → relay
  ├─ 涉及前端 UI 交互？ → bridge + frontend
  ├─ 涉及证书操作？ → certs
  ├─ 涉及 Cursor 编辑器？ → cursor
  └─ 跨模块功能？ → 在最上层包中协调
```

### 新增 Provider 的归属

新增 LLM Provider（如 Gemini、Mistral）应添加到 `internal/agent/` 包中，实现 `AdapterTarget` 接口。

### 新增路由规则的归属

新增 MITM 路由规则应添加到 `internal/mitm/` 包中。

### 新增模型映射的归属

新增模型映射模板应添加到 `internal/relay/` 包中。

## 代码审查职责

### 审查重点

| 模块 | 审查重点 |
|------|---------|
| `agent` | 会话安全性、Provider 兼容性、历史记录完整性 |
| `bridge` | 前后端接口一致性、状态管理正确性 |
| `mitm` | 路由规则正确性、TLS 安全性 |
| `relay` | 请求重写正确性、模型映射完整性 |
| `certs` | 证书安全性、跨平台兼容性 |
| `cursor` | 进程检测准确性、配置安全性 |
| `frontend` | UI 一致性、用户体验 |

### 审查流程

1. 代码作者提交 PR
2. 至少 1 名审查者 review
3. 审查者按上述重点检查
4. 提出修改意见或 approve
5. 作者修改后重新提交
6. 审查者 approve 后合并
