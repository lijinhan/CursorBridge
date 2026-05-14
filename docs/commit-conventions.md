# 代码提交规范

## Conventional Commits

所有 commit message 必须遵循 [Conventional Commits](https://www.conventionalcommits.org/) 格式：

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Type 列表

| Type | 说明 | 示例 |
|------|------|------|
| `feat` | 新功能 | `feat(agent): add Gemini provider support` |
| `fix` | Bug 修复 | `fix(relay): correct model name mapping for o1` |
| `refactor` | 重构（不改变行为） | `refactor(bridge): extract config logic to separate package` |
| `docs` | 文档变更 | `docs: add coding standards` |
| `test` | 测试相关 | `test(agent): add session lifecycle tests` |
| `chore` | 构建/工具变更 | `chore: update Go to 1.23` |
| `ci` | CI/CD 配置 | `ci: add GitHub Actions release workflow` |
| `build` | 构建系统变更 | `build: update Wails to v3` |
| `style` | 代码格式（不影响逻辑） | `style: fix import ordering` |
| `perf` | 性能优化 | `perf(mitm): reduce memory allocation in proxy handler` |

### Scope 列表

| Scope | 对应模块 |
|-------|---------|
| `agent` | `internal/agent/` |
| `bridge` | `internal/bridge/` |
| `mitm` | `internal/mitm/` |
| `relay` | `internal/relay/` |
| `certs` | `internal/certs/` |
| `cursor` | `internal/cursor/` |
| `frontend` | `frontend/` |
| `docs` | `docs/` |
| `build` | `build/`, `wails.json`, `Makefile` |

### 规则

1. **description** 使用英文，现在时态，首字母小写，不加句号
2. **body** 可选，用于说明变更原因和影响范围
3. **Breaking Changes** 必须在 footer 中标注：
   ```
   feat(relay): change gateway API signature

   BREAKING CHANGE: Gateway.New now requires a Config parameter
   ```
4. 单个 commit 只做一件事
5. 禁止提交包含调试代码、注释掉的代码、或临时代码

### 示例

```
feat(agent): add Gemini provider support

Implement Google Gemini API adapter following the same pattern
as OpenAI and Anthropic adapters. Supports streaming responses
and tool use.

Refs: #42
```

```
fix(mitm): handle TLS handshake timeout gracefully

Previously, a slow TLS handshake would block the proxy goroutine
indefinitely. Now respects context cancellation.

Fixes: #38
```

```
refactor: restructure agent package into sub-packages

Split the monolithic agent package into focused sub-packages:
- session: session lifecycle management
- provider: LLM provider adapters
- history: conversation history
- loop: agent run loop

No behavior changes.
```

## PR 规范

### PR 标题

与 commit message 格式一致：`type(scope): description`

### PR 描述模板

```markdown
## Summary

<!-- 1-3 句话描述变更内容 -->

## Changes

<!-- 具体变更列表 -->

- [ ] Change 1
- [ ] Change 2

## Test Plan

<!-- 如何验证变更 -->

- [ ] `go build ./...` 编译通过
- [ ] `go vet ./...` 无警告
- [ ] 手动测试场景

## Related Issues

<!-- 关联的 Issue 编号 -->
```

### PR 审查

- 至少 1 人 approve 后方可合并
- CI 检查全部通过
- 无 merge conflict
- Squash merge 到 main 分支
