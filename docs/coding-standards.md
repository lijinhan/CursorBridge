# 编码规范

## Go 代码规范

### 包命名

- 全小写，单个单词，不使用下划线或驼峰
- 例：`agent`, `bridge`, `mitm`, `relay`
- 禁止：`agentLoop`, `agent_loop`, `proxyService`

### 文件命名

- 全小写，下划线分隔
- 例：`proxy_service.go`, `background_composer.go`
- 测试文件：`*_test.go`
- 平台特定文件：`*_darwin.go`, `*_windows.go`, `*_linux.go`

### 函数命名

- 导出函数：大驼峰（PascalCase）— `NewProxyService`, `HandleRunSSE`
- 未导出函数：小驼峰（camelCase）— `readConfig`, `buildCAWarning`
- 构造函数统一用 `New` 前缀：`NewProxyService()`, `NewGateway()`

### 类型命名

- 导出类型：大驼峰 — `ProxyState`, `AdapterInfo`
- 未导出类型：小驼峰 — `sessionStore`, `toolResultEnvelope`
- 接口：大驼峰，单方法接口用 `-er` 后缀 — `AdapterResolver`

### 错误处理

- 使用 `fmt.Errorf` 创建错误消息，包含上下文信息
- 错误消息格式：`包名.函数名: 具体描述`
  ```go
  return fmt.Errorf("agent.HandleRunSSE: no BYOK adapter configured")
  ```
- 禁止使用 `panic` 处理可恢复错误
- 禁止忽略 error 返回值（除非有明确理由并添加注释）

### 注释规范

- 每个导出函数/类型必须有 godoc 注释
- 注释以函数名开头：
  ```go
  // HandleRunSSE streams the BYOK chat completion to w as Connect SSE frames.
  func HandleRunSSE(...) { ... }
  ```
- 注释说明 **意图**，而非 **实现**
- 不写重复代码逻辑的注释

### 常量与变量

- 常量使用 `const`，分组定义：
  ```go
  const (
      defaultListenAddr = "127.0.0.1:18080"
      defaultUpstream   = "https://api2.cursor.sh"
  )
  ```
- 全局变量必须有明确的初始化和注释

### 并发安全

- 共享状态必须使用 `sync.Mutex` 或 `sync.RWMutex` 保护
- 优先使用 channel 传递数据，而非共享内存
- goroutine 必须有明确的退出机制（context cancel / done channel）

### Import 顺序

```go
import (
    // 标准库
    "context"
    "fmt"
    "net/http"

    // 项目内部包
    "cursor-byok/internal/agent"
    "cursor-byok/internal/certs"

    // 第三方库
    "github.com/wailsapp/wails/v3/pkg/application"
)
```

## Vue / TypeScript 规范

### 组件命名

- PascalCase：`ProxyDashboard.vue`, `ModelConfig.vue`
- 单文件组件（SFC）结构顺序：`<script>` → `<template>` → `<style>`

### 类型定义

- 接口和类型使用 PascalCase
- 枚举使用 PascalCase
- 变量和函数使用 camelCase

### 样式

- 使用 scoped style
- CSS 类名使用 kebab-case：`proxy-state`, `model-config`

## 通用规范

### 文件编码

- UTF-8 编码
- LF 换行符（非 CRLF）
- 文件末尾保留一个空行

### 行宽

- Go 代码：120 字符（gofmt 默认）
- Vue/TypeScript：100 字符
- Markdown 文档：无限制

### 禁止事项

1. 禁止提交包含硬编码密钥或 token 的代码
2. 禁止使用 `// TODO: fix later` 等模糊注释，必须说明具体问题
3. 禁止在循环中使用 `defer`
4. 禁止在导出函数中返回未导出类型
5. 禁止使用 `_ = err` 忽略错误（除非有明确注释说明原因）
6. 禁止在 `internal/` 包中导出仅被同包内使用的函数
