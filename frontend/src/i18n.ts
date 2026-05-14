// Reactive i18n module: locale → translations map, with a Vue-aware $t function.
// Supports zh-CN (default) and en. Add keys as needed.
//
// Usage in components:
//   import { t, locale } from '../i18n'
//   <span>{{ t('brand.name') }}</span>
//   locale.value = 'en'  // triggers re-render

import { ref, computed } from 'vue'

type TranslationMap = Record<string, string>;

const zhCN: TranslationMap = {
  // Brand / topbar
  "brand.name": "CursorBridge",
  "brand.sub": "本地 MITM · BYOK 网关",
  "status.running": "运行中",
  "status.stopped": "已停止",
  "btn.start": "启动服务",
  "btn.stop": "停止",

  // Tabs
  "tab.overview": "总览",
  "tab.models": "模型",
  "tab.stats": "统计",

  // Overview
  "overview.models": "模型",
  "overview.addModel": "+ 添加模型",
  "overview.noModels": "暂无已配置的模型。",
  "overview.unnamed": "未命名",

  // Settings
  "settings.activeModel": "默认活动模型",
  "settings.activeModelDesc": "控制新聊天、cmd-K 和 Agent 使用的模型。",
  "settings.newChats": "新聊天",
  "settings.firstModel": "第一个配置的模型",
  "settings.specializedRouting": "专用模型路由",
  "settings.commitGen": "提交消息生成",
  "settings.codeReview": "代码审查",
  "settings.defaultModel": "默认模型",
  "settings.trustCA": "信任本地 CA",
  "settings.installed": "已安装",
  "settings.notInstalled": "未安装",
  "settings.installCA": "安装 CA",
  "settings.uninstallCA": "卸载 CA",
  "settings.loopLimits": "Agent 循环限制",
  "settings.zeroNative": "0 = 不限制",
  "settings.maxRounds": "最大轮次",
  "settings.maxDuration": "最大时长 (分钟)",
  "settings.settingsFolder": "配置目录",
  "settings.configDesc": "你的配置和 CA 证书存放在这里。",
  "settings.openFolder": "打开目录",
  "settings.osWindows": "Windows 版本",
  "settings.osMac": "macOS 版本",
  "settings.osLinux": "Linux 版本",
  "settings.osDesktop": "桌面版本",

  // Models page
  "models.testAll": "全部测试",
  "models.addModel": "+ 添加模型",
  "models.noModels": "暂无 {provider} 模型",
  "models.addFirst": "+ 添加第一个模型",
  "models.addFirstDesc": "添加模型以路由 BYOK 请求到你的 API 密钥。",
  "models.test": "测试",
  "models.edit": "编辑",
  "models.duplicate": "复制",
  "models.delete": "删除",
  "models.copy": " (复制)",
  "models.healthy": "正常",
  "models.error": "错误",
  "models.untested": "未测试",
  "models.host": "主机",
  "models.apiKey": "API 密钥",
  "models.testing": "测试中…",

  // Stats
  "stats.title": "词元用量",
  "stats.desc": "从磁盘对话历史聚合计算。",
  "stats.totalTokens": "总词元",
  "stats.prompt": "提示词",
  "stats.input": "输入",
  "stats.completion": "补全",
  "stats.output": "输出",
  "stats.conversations": "对话",
  "stats.turns": "轮次",
  "stats.last7days": "最近 7 天",
  "stats.last7daysDesc": "每日提示词与补全词元对比。",
  "stats.noUsage": "最近一周无使用记录。",
  "stats.perModel": "按模型",
  "stats.perModelDesc": "按总词元排序。",
  "stats.noTurns": "暂无轮次记录。",
  "stats.colModel": "模型",
  "stats.colProvider": "提供商",
  "stats.colPrompt": "提示词",
  "stats.colCompletion": "补全",
  "stats.colTotal": "总计",
  "stats.colTurns": "轮次",

  // Editor
  "editor.new": "新建模型",
  "editor.edit": "编辑模型",
  "editor.cancel": "取消",
  "editor.save": "保存",
  "editor.saveAndTest": "保存并测试",
  "editor.identity": "标识",
  "editor.identityDesc": "在选择器中显示的标签。自由填写。",
  "editor.displayName": "显示名称",
  "editor.modelID": "模型 ID",
  "editor.endpoint": "端点与凭据",
  "editor.endpointDesc": "不会离开本机。本地存储于 config.json。",
  "editor.apiKey": "API 密钥",
  "editor.hideKey": "隐藏密钥",
  "editor.showKey": "显示密钥",
  "editor.baseURL": "基础 URL",
  "editor.advanced": "高级设置",
  "editor.advancedDesc": "全部可选。留空使用提供商默认值。",
  "editor.contextWindow": "上下文窗口",
  "editor.reasoningEffort": "推理力度",
  "editor.fastMode": "快速模式",
  "editor.maxOutput": "最大输出词元",
  "editor.thinkingBudget": "思考预算",
  "editor.retryCount": "重试次数",
  "editor.retryInterval": "重试间隔 (毫秒)",
  "editor.timeout": "超时 (毫秒)",
  "editor.notes": "备注",
  "editor.testResult": "测试结果",
  "editor.testResultDesc": "最近一次探测结果。",
  "editor.noTest": "尚未测试 — 使用「保存并测试」。",
  "editor.healthy": "正常 — 适配器响应成功。",
  "editor.back": "← 模型",

  // Close dialog
  "close.title": "关闭 CursorBridge？",
  "close.desc": "你可以完全退出应用，或最小化到系统托盘继续运行。",
  "close.quit": "退出",
  "close.tray": "最小化到托盘",
  "close.remember": "记住我的选择",

  // Footer
  "footer.checkUpdates": "检查更新",
  "footer.checking": "检查中…",
  "footer.updateAvailable": "发现新版本 {version}！",
  "footer.noUpdate": "当前已是最新版本。",
  "footer.updateError": "检查更新失败：{error}",
  "footer.confirmUpdate": "立即更新？应用将重启。",
  "footer.cannotOpenBrowser": "无法打开浏览器：{error}",
  "footer.refreshState": "从后端重新加载状态",

  // Help tooltips
  "help.cacheHit": "从上游缓存而非模型直接返回的请求比例。",
  "help.turns": "本次会话中的助手轮次总数。无效轮次为出错或被取消的轮次。",
  "help.tokens": "本次会话中的提示词 + 补全词元总数。",
  "help.listen": "CursorBridge 接受 Cursor IDE 代理流量的本地地址。",
  "help.uptime": "本地代理自上次启动以来的运行时长。",
  "help.caFp": "本地 CA 的 SHA-256 指纹。启用 MITM 前请将此 CA 安装到平台信任存储中。",
  "help.displayName": "在选择器中显示的标签。自由填写，不会发送给提供商。",
  "help.modelID": "发送给上游提供商的规范模型标识符（如 gpt-4o、claude-sonnet-4-5）。",
  "help.apiKey": "提供商 API 密钥。本地存储于 config.json 中，不会在 BYOK 调用之外传输。",
  "help.baseURL": "提供商端点。可覆盖为代理/反向代理/Azure OpenAI 等。",
  "help.contextWindow": "发送给提供商的最大输入词元数。留空使用提供商默认值。",
  "help.reasoningEffort": "OpenAI 推理模型（o1、o3、gpt-5 系列）的推理预算提示。",
  "help.fastMode": "使用优先服务层级以获得更快响应（OpenAI）。每词元费用更高。",
  "help.maxOutput": "每次响应的输出词元上限。留空使用提供商默认值。",
  "help.thinkingBudget": "Anthropic 扩展思考词元预算。仅适用于支持推理的模型。",
  "help.retryCount": "上游请求失败时的最大重试次数。留空使用默认值 2（推荐）。适用于 429/5xx 等可重试错误。",
  "help.retryInterval": "重试间隔基础值（毫秒）。实际间隔按指数退避递增，上限 60 秒。留空使用默认值 1000。",
  "help.timeout": "单次上游请求超时时间（毫秒）。留空使用默认值 300000（5 分钟）。",
  "help.notes": "私人备注。不会发送到任何地方。",
  "help.maxLoopRounds": "Agent 工具调用循环的最大轮次。0 = 不限制（沿用原生 Cursor 体验，由客户端控制何时停止）。",
  "help.maxTurnDurationMin": "每次 Agent 会话的最大时长（分钟）。0 = 不限制（沿用原生体验）。",
};

const en: TranslationMap = {
  // Brand / topbar
  "brand.name": "CursorBridge",
  "brand.sub": "Local MITM · BYOK Gateway",
  "status.running": "Running",
  "status.stopped": "Stopped",
  "btn.start": "Start",
  "btn.stop": "Stop",

  // Tabs
  "tab.overview": "Overview",
  "tab.models": "Models",
  "tab.stats": "Stats",

  // Overview
  "overview.models": "Models",
  "overview.addModel": "+ Add model",
  "overview.noModels": "No models configured yet.",
  "overview.unnamed": "Unnamed",

  // Settings
  "settings.activeModel": "Default active model",
  "settings.activeModelDesc": "Controls which model new chats, cmd-K, and Agent use.",
  "settings.newChats": "New chats",
  "settings.firstModel": "First configured model",
  "settings.specializedRouting": "Specialized model routing",
  "settings.commitGen": "Commit generator",
  "settings.codeReview": "Code review",
  "settings.defaultModel": "Default model",
  "settings.trustCA": "Trust local CA",
  "settings.installed": "Installed",
  "settings.notInstalled": "Not installed",
  "settings.installCA": "Install CA",
  "settings.uninstallCA": "Uninstall CA",
  "settings.loopLimits": "Agent loop limits",
  "settings.zeroNative": "0 = native",
  "settings.maxRounds": "Max rounds",
  "settings.maxDuration": "Max duration (min)",
  "settings.settingsFolder": "Settings folder",
  "settings.configDesc": "Your config and CA live here.",
  "settings.openFolder": "Open folder",
  "settings.osWindows": "Windows version",
  "settings.osMac": "macOS version",
  "settings.osLinux": "Linux version",
  "settings.osDesktop": "Desktop version",

  // Models page
  "models.testAll": "Test all",
  "models.addModel": "+ Add model",
  "models.noModels": "No {provider} models yet",
  "models.addFirst": "+ Add your first model",
  "models.addFirstDesc": "Add a model to route BYOK requests to your API key.",
  "models.test": "Test",
  "models.edit": "Edit",
  "models.duplicate": "Duplicate",
  "models.delete": "Delete",
  "models.copy": " (copy)",
  "models.healthy": "Healthy",
  "models.error": "Error",
  "models.untested": "Untested",
  "models.host": "Host",
  "models.apiKey": "API key",
  "models.testing": "Testing…",

  // Stats
  "stats.title": "Token usage",
  "stats.desc": "Aggregated from on-disk conversation history.",
  "stats.totalTokens": "Total tokens",
  "stats.prompt": "Prompt",
  "stats.input": "input",
  "stats.completion": "Completion",
  "stats.output": "output",
  "stats.conversations": "Conversations",
  "stats.turns": "turns",
  "stats.last7days": "Last 7 days",
  "stats.last7daysDesc": "Prompt vs completion tokens per day.",
  "stats.noUsage": "No usage recorded in the last week.",
  "stats.perModel": "Per model",
  "stats.perModelDesc": "Sorted by total tokens.",
  "stats.noTurns": "No turns recorded yet.",
  "stats.colModel": "Model",
  "stats.colProvider": "Provider",
  "stats.colPrompt": "Prompt",
  "stats.colCompletion": "Completion",
  "stats.colTotal": "Total",
  "stats.colTurns": "Turns",

  // Editor
  "editor.new": "New model",
  "editor.edit": "Edit model",
  "editor.cancel": "Cancel",
  "editor.save": "Save",
  "editor.saveAndTest": "Save and test",
  "editor.identity": "Identity",
  "editor.identityDesc": "Label shown in the picker. Free-form.",
  "editor.displayName": "Display name",
  "editor.modelID": "Model ID",
  "editor.endpoint": "Endpoint & credentials",
  "editor.endpointDesc": "Never leaves this machine. Stored locally in config.json.",
  "editor.apiKey": "API key",
  "editor.hideKey": "Hide key",
  "editor.showKey": "Show key",
  "editor.baseURL": "Base URL",
  "editor.advanced": "Advanced",
  "editor.advancedDesc": "All optional. Leave blank for provider defaults.",
  "editor.contextWindow": "Context window",
  "editor.reasoningEffort": "Reasoning effort",
  "editor.fastMode": "Fast mode",
  "editor.maxOutput": "Max output tokens",
  "editor.thinkingBudget": "Thinking budget",
  "editor.retryCount": "Retry count",
  "editor.retryInterval": "Retry interval (ms)",
  "editor.timeout": "Timeout (ms)",
  "editor.notes": "Notes",
  "editor.testResult": "Test result",
  "editor.testResultDesc": "Last probe against this adapter.",
  "editor.noTest": "No test run yet — use Save and test.",
  "editor.healthy": "Healthy — adapter responded successfully.",
  "editor.back": "← Models",

  // Close dialog
  "close.title": "Close CursorBridge?",
  "close.desc": "You can fully quit the app, or minimize to system tray to keep it running.",
  "close.quit": "Quit",
  "close.tray": "Minimize to tray",
  "close.remember": "Remember my choice",

  // Footer
  "footer.checkUpdates": "Check for updates",
  "footer.checking": "Checking…",
  "footer.updateAvailable": "New version {version} available!",
  "footer.noUpdate": "You're on the latest version.",
  "footer.updateError": "Update check failed: {error}",
  "footer.confirmUpdate": "Update now? The app will restart.",
  "footer.cannotOpenBrowser": "Cannot open browser: {error}",
  "footer.refreshState": "Reload state from backend",

  // Help tooltips
  "help.cacheHit": "Fraction of requests served from upstream cache rather than the model directly.",
  "help.turns": "Total assistant turns in this session. Invalid turns are those that errored or were cancelled.",
  "help.tokens": "Total prompt + completion tokens in this session.",
  "help.listen": "Local address where CursorBridge accepts Cursor IDE proxy traffic.",
  "help.uptime": "Time since the local proxy was last started.",
  "help.caFp": "SHA-256 fingerprint of the local CA. Install this CA into the platform trust store before enabling MITM.",
  "help.displayName": "Label shown in the picker. Free-form, never sent to the provider.",
  "help.modelID": "Canonical model identifier sent to the upstream provider (e.g. gpt-4o, claude-sonnet-4-5).",
  "help.apiKey": "Provider API key. Stored locally in config.json, never transmitted outside BYOK calls.",
  "help.baseURL": "Provider endpoint. Can be overridden for proxies/reverse proxies/Azure OpenAI etc.",
  "help.contextWindow": "Maximum input tokens sent to the provider. Leave blank for provider default.",
  "help.reasoningEffort": "Reasoning budget hint for OpenAI reasoning models (o1, o3, gpt-5 series).",
  "help.fastMode": "Use priority service tier for faster responses (OpenAI). Higher per-token cost.",
  "help.maxOutput": "Per-response output token cap. Leave blank for provider default.",
  "help.thinkingBudget": "Anthropic extended thinking token budget. Only for models that support reasoning.",
  "help.retryCount": "Max retries on upstream request failure. Leave blank for default 2. Applies to 429/5xx etc.",
  "help.retryInterval": "Base retry interval in ms. Actual interval increases with exponential backoff, capped at 60s.",
  "help.timeout": "Per-request upstream timeout in ms. Leave blank for default 5 minutes.",
  "help.notes": "Private notes. Never sent anywhere.",
  "help.maxLoopRounds": "Max rounds for the agent tool-call loop. 0 = no limit (native Cursor experience).",
  "help.maxTurnDurationMin": "Max duration per agent session in minutes. 0 = no limit (native experience).",
};

const locales: Record<string, TranslationMap> = { "zh-CN": zhCN, en };

// Reactive locale ref — changing this triggers Vue re-renders for any
// computed that depends on it.
export const locale = ref<string>("zh-CN");

// Reactive t() — returns a computed string that updates when locale changes.
// In templates: {{ t('key') }} re-renders on locale switch.
export function t(key: string, params?: Record<string, string>): string {
  const map = locales[locale.value] ?? locales["zh-CN"];
  let text = map[key] ?? locales["zh-CN"]?.[key] ?? key;
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      text = text.replace(`{${k}}`, v);
    }
  }
  return text;
}

// Make t() reactive by wrapping it in a computed that tracks locale.
// Components should use `useT()` which returns a reactive translation function.
export function useT() {
  return computed(() => (key: string, params?: Record<string, string>) => t(key, params));
}

export function setLocale(l: string) {
  locale.value = l in locales ? l : "zh-CN";
}

export function getLocale(): string {
  return locale.value;
}