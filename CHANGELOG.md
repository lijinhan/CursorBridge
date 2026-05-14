# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed
- **Breaking**: README 重写，原作者信息移至 Credits/致谢部分，项目定位为独立项目
- 重构项目文档体系，新增 `docs/` 目录及 6 个规范文档
- 新增 `.editorconfig` 统一编辑器配置

### Added
- `docs/architecture.md` — 系统架构设计（含 Mermaid 图）
- `docs/directory-structure.md` — 目录结构与职责说明
- `docs/coding-standards.md` — 编码规范（Go / Vue / TypeScript）
- `docs/commit-conventions.md` — 代码提交规范（Conventional Commits）
- `docs/release-process.md` — 发布流程（SemVer + 检查清单）
- `docs/responsibility.md` — 职责划分与依赖规则
- `.editorconfig` — 编辑器配置统一

## [0.1.0] - 2025-06-01

### Added
- BYOK 代理核心功能
- OpenAI / Anthropic Provider 支持
- MITM 代理与本地 CA 证书管理
- Wails 桌面管理界面
- 用量统计功能
- Cursor 编辑器集成
