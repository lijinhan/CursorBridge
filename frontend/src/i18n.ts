// Minimal i18n module: locale → translations map, with a reactive $t function.
// Supports zh-CN (default) and en. Add keys as needed.

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
  "settings.trustCA": "Trust local CA",
  "settings.loopLimits": "Agent loop limits",
  "settings.settingsFolder": "Settings folder",
  "settings.openFolder": "Open folder",

  // Models page
  "models.testAll": "Test all",
  "models.addModel": "+ 添加模型",
  "models.noModels": "No {provider} models yet",
  "models.addFirst": "+ Add your first model",
  "models.test": "Test",
  "models.edit": "Edit",
  "models.duplicate": "Duplicate",
  "models.delete": "Delete",
  "models.copy": " (复制)",

  // Stats
  "stats.title": "Token usage",
  "stats.totalTokens": "Total tokens",
  "stats.prompt": "Prompt",
  "stats.completion": "Completion",
  "stats.conversations": "Conversations",
  "stats.last7days": "Last 7 days",
  "stats.perModel": "Per model",

  // Editor
  "editor.new": "New model",
  "editor.edit": "Edit model",
  "editor.cancel": "Cancel",
  "editor.save": "Save",
  "editor.saveAndTest": "Save and test",
  "editor.identity": "Identity",
  "editor.endpoint": "Endpoint & credentials",
  "editor.advanced": "Advanced",
  "editor.testResult": "Test result",

  // Close dialog
  "close.title": "Close CursorBridge?",
  "close.quit": "Quit",
  "close.tray": "Minimize to tray",
  "close.remember": "Remember my choice",

  // Footer
  "footer.checkUpdates": "Check for updates",
  "footer.checking": "Checking…",

  // Help tooltips (Chinese)
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
  "brand.name": "CursorBridge",
  "brand.sub": "Local MITM · BYOK Gateway",
  "status.running": "Running",
  "status.stopped": "Stopped",
  "btn.start": "Start",
  "btn.stop": "Stop",
  "tab.overview": "Overview",
  "tab.models": "Models",
  "tab.stats": "Stats",
  "overview.models": "Models",
  "overview.addModel": "+ Add model",
  "overview.noModels": "No models configured yet.",
  "overview.unnamed": "Unnamed",
  "settings.activeModel": "Default active model",
  "settings.trustCA": "Trust local CA",
  "settings.loopLimits": "Agent loop limits",
  "settings.settingsFolder": "Settings folder",
  "settings.openFolder": "Open folder",
  "models.testAll": "Test all",
  "models.addModel": "+ Add model",
  "models.noModels": "No {provider} models yet",
  "models.addFirst": "+ Add your first model",
  "models.test": "Test",
  "models.edit": "Edit",
  "models.duplicate": "Duplicate",
  "models.delete": "Delete",
  "models.copy": " (copy)",
  "stats.title": "Token usage",
  "stats.totalTokens": "Total tokens",
  "stats.prompt": "Prompt",
  "stats.completion": "Completion",
  "stats.conversations": "Conversations",
  "stats.last7days": "Last 7 days",
  "stats.perModel": "Per model",
  "editor.new": "New model",
  "editor.edit": "Edit model",
  "editor.cancel": "Cancel",
  "editor.save": "Save",
  "editor.saveAndTest": "Save and test",
  "editor.identity": "Identity",
  "editor.endpoint": "Endpoint & credentials",
  "editor.advanced": "Advanced",
  "editor.testResult": "Test result",
  "close.title": "Close CursorBridge?",
  "close.quit": "Quit",
  "close.tray": "Minimize to tray",
  "close.remember": "Remember my choice",
  "footer.checkUpdates": "Check for updates",
  "footer.checking": "Checking…",
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
  "help.retryCount": "Max retries on upstream request failure (0 = no retry). Applies to 429/5xx etc.",
  "help.retryInterval": "Base retry interval in ms. Actual interval increases with exponential backoff, capped at 60s.",
  "help.timeout": "Per-request upstream timeout in ms. Leave blank for default 5 minutes.",
  "help.notes": "Private notes. Never sent anywhere.",
  "help.maxLoopRounds": "Max rounds for the agent tool-call loop. 0 = no limit (native Cursor experience, client controls when to stop).",
  "help.maxTurnDurationMin": "Max duration per agent session in minutes. 0 = no limit (native experience).",
};

const locales: Record<string, TranslationMap> = { "zh-CN": zhCN, en };
let currentLocale = "zh-CN";

export function setLocale(locale: string) {
  currentLocale = locale in locales ? locale : "zh-CN";
}

export function getLocale(): string {
  return currentLocale;
}

export function t(key: string, params?: Record<string, string>): string {
  let text = locales[currentLocale]?.[key] ?? locales["zh-CN"]?.[key] ?? key;
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      text = text.replace(`{${k}}`, v);
    }
  }
  return text;
}
