export type ProxyState = {
  running: boolean;
  listenAddr: string;
  baseURL: string;
  startedAt: number;
  caFingerprint: string;
  caPath: string;
  caInstalled: boolean;
  caInstallMode?: string;
  caWarning?: string;
  lastError?: string;
};

export type ModelAdapter = {
  displayName: string;
  type: string;
  baseURL: string;
  apiKey: string;
  modelID: string;
  contextWindow?: string;
  reasoningEffort?: string;
  serviceTier?: string;
  maxOutputTokens?: number;
  thinkingBudget?: number;
  retryCount?: number;
  retryInterval?: number;
  timeout?: number;
  notes?: string;
  lastTestResult?: string;
  lastTestedAt?: number;
};

export type UserConfig = {
  baseURL: string;
  modelAdapters: ModelAdapter[];
  activeModelID?: string;
  commitModelID?: string;
  reviewModelID?: string;
  proxyPort?: number;
  maxLoopRounds?: number;
  maxTurnDurationMin?: number;
};

export type CursorTweaks = {
  path: string;
  found: boolean;
  error?: string;
  proxySet: boolean;
  proxyValue?: string;
  strictSSLOff: boolean;
  proxySupportOn: boolean;
  systemCertsV2On: boolean;
  useHttp1: boolean;
  disableHttp2: boolean;
  proxyKerberos: boolean;
};

export type View = "overview" | "models" | "stats" | "editor";
export type Provider = "openai" | "anthropic";

export type ModelUsageEntry = {
  model: string;
  provider: string;
  promptTokens: number;
  completionTokens: number;
  turnCount: number;
};

export type DailyUsageEntry = {
  date: string;
  promptTokens: number;
  completionTokens: number;
};

export type UsageStats = {
  totalPromptTokens: number;
  totalCompletionTokens: number;
  totalTokens: number;
  conversationCount: number;
  turnCount: number;
  perModel: ModelUsageEntry[];
  last7Days: DailyUsageEntry[];
};

export type UpdateInfo = {
  hasUpdate: boolean;
  currentTag: string;
  latestTag: string;
  downloadURL?: string;
  filename?: string;
  fileSize?: number;
  sha256URL?: string;
  changelogURL?: string;
};

export type DownloadProgress = {
  downloaded: number;
  total: number;
  percent: number;
  path: string;
};

export type UpdateState = {
  checking: boolean;
  available: boolean;
  downloading: boolean;
  installing: boolean;
  info?: UpdateInfo;
  progress?: DownloadProgress;
  error?: string;
};