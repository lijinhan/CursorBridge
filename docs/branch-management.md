# Git 分支管理规范

## 分支模型

采用简化的 Git Flow 模型，以 `main` 为稳定主线。

## 分支类型

| 分支格式 | 用途 | 生命周期 | 示例 |
|---------|------|---------|------|
| `main` | 稳定发布线 | 永久 | `main` |
| `feat/<name>` | 新功能开发 | 合并后删除 | `feat/dynamic-context-token-limit` |
| `fix/<name>` | Bug 修复 | 合并后删除 | `fix/sse-frame-encoding` |
| `refactor/<name>` | 代码重构 | 合并后删除 | `refactor/deps-injection` |
| `docs/<name>` | 文档更新 | 合并后删除 | `docs/project-documentation` |
| `release/vX.Y.Z` | 版本发布准备 | 发布后删除 | `release/v0.2.0` |
| `hotfix/vX.Y.Z` | 紧急修复 | 合并后删除 | `hotfix/v0.1.1` |

## 分支命名规范

- 使用小写英文，单词间用 `-` 连接
- 名称应简明扼要地描述变更内容
- issue 相关的分支可在 PR 描述中关联 issue 编号

## 工作流程

### 功能开发

```
main ──→ feat/my-feature ──→ PR ──→ main
```

1. 从 `main` 创建 `feat/<name>` 分支
2. 开发并提交（遵循 Conventional Commits）
3. 推送到远程并创建 PR
4. PR 审核通过后 squash merge 到 `main`
5. 删除功能分支

### Bug 修复

```
main ──→ fix/bug-name ──→ PR ──→ main
```

流程与功能开发相同。

### 版本发布

```
main ──→ release/vX.Y.Z ──→ tag ──→ main
```

1. 从 `main` 创建 `release/vX.Y.Z` 分支
2. 更新版本号和 CHANGELOG
3. 提交版本变更
4. 打 tag `vX.Y.Z`
5. 合并回 `main`
6. 基于 tag 构建并发布 GitHub Release
7. 删除 release 分支

### 紧急修复

```
main ──→ hotfix/vX.Y.Z ──→ tag ──→ main
```

1. 从 `main` 创建 `hotfix/vX.Y.Z` 分支
2. 修复并验证
3. 更新版本号（PATCH +1）
4. 打 tag 并发布
5. 合并回 `main`

## PR 合并规范

- **审核要求**：至少一人审核通过
- **CI 检查**：所有 CI 检查必须通过
- **冲突处理**：无合并冲突
- **合并方式**：Squash merge 到 `main`
- **PR 标题**：遵循 Conventional Commits 格式
- **PR 描述**：包含 Summary、Test Plan、Related Issues

## Tag 规范

- 格式：`v主版本.次版本.修订号`（如 `v0.1.0`）
- 遵循 Semantic Versioning 2.0.0
- Tag 触发自动化发布流程

## 保护规则

- `main` 分支受保护，禁止直接推送
- 所有变更必须通过 PR
- PR 必须通过 CI 检查才能合并
