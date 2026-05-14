# 发布流程

## 版本号规则

遵循 [Semantic Versioning 2.0.0](https://semver.org/)：

```
MAJOR.MINOR.PATCH
```

- **MAJOR**: 不兼容的 API 变更（Breaking Changes）
- **MINOR**: 向后兼容的新功能
- **PATCH**: 向后兼容的 Bug 修复

预发布版本：`1.2.0-alpha.1`, `1.2.0-beta.2`, `1.2.0-rc.1`

## 发布前检查清单

- [ ] 所有测试通过：`go test ./...`
- [ ] 静态分析无警告：`go vet ./...`
- [ ] 前端构建通过：`cd frontend && npm run build`
- [ ] 生产构建通过：`wails build`
- [ ] CHANGELOG.md 已更新
- [ ] 版本号已在以下位置更新：
  - [ ] `build/windows/info.json` → `ProductVersion`
  - [ ] `build/darwin/Info.plist` → `CFBundleShortVersionString`
  - [ ] `wails.json` (如有版本字段)
- [ ] 无未提交的变更
- [ ] 无未合并的 feature 分支

## 发布步骤

### 1. 准备发布分支

```bash
git checkout main
git pull origin main
git checkout -b release/vX.Y.Z
```

### 2. 更新版本号和 CHANGELOG

```bash
# 编辑 CHANGELOG.md，将 [Unreleased] 替换为版本号和日期
# 更新所有版本号文件
```

### 3. 提交版本变更

```bash
git add .
git commit -m "chore: bump version to vX.Y.Z"
```

### 4. 打标签

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
```

### 5. 合并到 main

```bash
git checkout main
git merge release/vX.Y.Z
git push origin main --tags
```

### 6. 构建 Release

```bash
# 多平台构建
wails build -platform darwin/amd64
wails build -platform darwin/arm64
wails build -platform windows/amd64
wails build -platform linux/amd64
```

### 7. 创建 GitHub Release

```bash
gh release create vX.Y.Z \
  --title "vX.Y.Z" \
  --notes-file CHANGELOG.md \
  build/bin/*
```

### 8. 清理

```bash
git branch -d release/vX.Y.Z
```

## CHANGELOG 维护规范

CHANGELOG.md 格式遵循 [Keep a Changelog](https://keepachangelog.com/)：

```markdown
# Changelog

## [Unreleased]

## [1.2.0] - 2026-05-12

### Added
- Gemini provider support

### Changed
- Improved retry logic with exponential backoff

### Fixed
- TLS handshake timeout handling

### Deprecated
- Old config format (will be removed in v2.0)

### Removed
- Legacy API endpoint support

### Security
- Updated CA certificate generation
```

## 热修复流程

紧急 Bug 修复可跳过部分检查：

1. 从 main 创建 `hotfix/vX.Y.Z+1` 分支
2. 修复 Bug，更新 CHANGELOG
3. 提交、打标签、推送
4. 合并回 main
5. 发布 PATCH 版本

## 回滚流程

如果发布后发现严重问题：

1. 在 GitHub Release 页面标记为 Pre-release
2. 创建 hotfix 分支修复
3. 发布新的 PATCH 版本
4. 更新文档说明已知问题
