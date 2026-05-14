import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { ProxyService } from "../../bindings/cursorbridge/internal/bridge";
import type { ProxyState, ModelAdapter, UserConfig, CursorTweaks, Provider, UsageStats } from "../types";

export const useProxyStore = defineStore("proxy", () => {
  const state = ref<ProxyState>({
    running: false,
    listenAddr: "",
    baseURL: "",
    startedAt: 0,
    caFingerprint: "",
    caPath: "",
    caInstalled: false,
  });
  const cfg = ref<UserConfig>({ baseURL: "", modelAdapters: [], activeModelID: "" });
  const tweaks = ref<CursorTweaks>({
    path: "",
    found: false,
    proxySet: false,
    strictSSLOff: false,
    proxySupportOn: false,
    systemCertsV2On: false,
    useHttp1: false,
    disableHttp2: false,
    proxyKerberos: false,
  });
  const busy = ref(false);
  const caBusy = ref(false);
  const providerTab = ref<Provider>("openai");
  const stats = ref<UsageStats>({
    totalPromptTokens: 0,
    totalCompletionTokens: 0,
    totalTokens: 0,
    conversationCount: 0,
    turnCount: 0,
    perModel: [],
    last7Days: [],
  });
  const statsLoading = ref(false);

  const filteredAdapters = computed(() =>
    cfg.value.modelAdapters
      .map((a, i) => ({ a, i }))
      .filter((x) => x.a.type === providerTab.value),
  );
  function adapterCountByType(type: string): number {
    return cfg.value.modelAdapters.filter((a) => a.type === type).length;
  }
  const modelOptions = computed(() =>
    cfg.value.modelAdapters.map((a, i) => ({
      value: a.modelID,
      label: `${a.displayName || `模型 ${i + 1}`} (${a.modelID || "—"})`,
    })),
  );
  const shortFP = computed(() =>
    state.value.caFingerprint
      ? state.value.caFingerprint.slice(0, 10) + "…" + state.value.caFingerprint.slice(-6)
      : "—",
  );
  const allTweaksOn = computed(() =>
    tweaks.value.proxySet &&
    tweaks.value.strictSSLOff &&
    tweaks.value.proxySupportOn &&
    tweaks.value.systemCertsV2On &&
    tweaks.value.useHttp1 &&
    tweaks.value.disableHttp2 &&
    tweaks.value.proxyKerberos,
  );
  const maxDailyTotal = computed(() => {
    let m = 0;
    for (const d of stats.value.last7Days) {
      const t = d.promptTokens + d.completionTokens;
      if (t > m) m = t;
    }
    return m;
  });

  async function refresh() {
    state.value = (await ProxyService.GetState()) as ProxyState;
    const c = (await ProxyService.LoadUserConfig()) as UserConfig;
    if (!c.modelAdapters) c.modelAdapters = [];
    if (!c.activeModelID) c.activeModelID = "";
    if (!c.commitModelID) c.commitModelID = "";
    if (!c.reviewModelID) c.reviewModelID = "";
    cfg.value = c;
    tweaks.value = (await ProxyService.GetCursorSettingsStatus()) as CursorTweaks;
  }

  async function loadStats() {
    statsLoading.value = true;
    try {
      stats.value = (await ProxyService.GetUsageStats()) as UsageStats;
    } finally {
      statsLoading.value = false;
    }
  }

  async function toggleService() {
    busy.value = true;
    try {
      state.value = state.value.running
        ? ((await ProxyService.StopProxy()) as ProxyState)
        : ((await ProxyService.StartProxy()) as ProxyState);
    } finally {
      busy.value = false;
    }
  }

  async function persistConfig() {
    await ProxyService.SaveUserConfig(cfg.value);
  }

  async function toggleCAInstall() {
    caBusy.value = true;
    try {
      state.value = state.value.caInstalled
        ? ((await ProxyService.UninstallCA()) as ProxyState)
        : ((await ProxyService.InstallCA()) as ProxyState);
    } finally {
      caBusy.value = false;
    }
  }

  async function applyTweaks() {
    tweaks.value = (await ProxyService.ApplyCursorTweaks()) as CursorTweaks;
  }

  async function revertTweaks() {
    tweaks.value = (await ProxyService.RevertCursorTweaks()) as CursorTweaks;
  }

  async function testAdapter(i: number) {
    const updated = (await ProxyService.TestAdapter(i)) as ModelAdapter;
    cfg.value.modelAdapters[i] = updated;
  }

  async function testAll() {
    for (let i = 0; i < cfg.value.modelAdapters.length; i++) {
      if (cfg.value.modelAdapters[i].type === providerTab.value) {
        await testAdapter(i);
      }
    }
  }

  async function duplicate(i: number) {
    const dup = JSON.parse(JSON.stringify(cfg.value.modelAdapters[i])) as ModelAdapter;
    dup.displayName += " (复制)";
    cfg.value.modelAdapters.push(dup);
    await persistConfig();
  }

  async function removeAdapter(i: number) {
    const removed = cfg.value.modelAdapters[i];
    cfg.value.modelAdapters.splice(i, 1);
    if (removed && cfg.value.activeModelID === removed.modelID) cfg.value.activeModelID = "";
    if (removed && cfg.value.commitModelID === removed.modelID) cfg.value.commitModelID = "";
    if (removed && cfg.value.reviewModelID === removed.modelID) cfg.value.reviewModelID = "";
    await persistConfig();
  }

  function clearError() {
    state.value.lastError = undefined;
  }

  return {
    state, cfg, tweaks, busy, caBusy, providerTab, stats, statsLoading,
    filteredAdapters, adapterCountByType, modelOptions, shortFP,
    allTweaksOn, maxDailyTotal,
    refresh, loadStats, toggleService, persistConfig, toggleCAInstall,
    applyTweaks, revertTweaks, testAdapter, testAll, duplicate, removeAdapter,
    clearError,
  };
});