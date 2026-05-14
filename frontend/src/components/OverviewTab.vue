<script setup lang="ts">
import { computed, ref } from "vue";
import { ProxyService } from "../../bindings/cursorbridge/internal/bridge";
import { Browser, System } from "@wailsio/runtime";
import { useProxyStore } from "../stores/proxy";
import { t } from "../i18n";
import OpenAIMark from "./logos/OpenAIMark.vue";
import AnthropicMark from "./logos/AnthropicMark.vue";

const store = useProxyStore();

const APP_VERSION = "1.0.1";
const GITHUB_REPO = "lijinhan/CursorBridge";
const GITHUB_URL = `https://github.com/${GITHUB_REPO}`;

const OS_LABEL = (() => {
  try {
    if (System.IsWindows()) return "Windows 版本";
    if (System.IsMac()) return "macOS 版本";
    if (System.IsLinux()) return "Linux 版本";
  } catch {
    /* runtime not ready (dev tools preview) — fall through */
  }
  return "桌面版本";
})();

const updateBusy = ref(false);

const HELP = {
  maxLoopRounds:
    "Agent 工具调用循环的最大轮次。0 = 不限制（沿用原生 Cursor 体验，由客户端控制何时停止）。",
  maxTurnDurationMin:
    "每次 Agent 会话的最大时长（分钟）。0 = 不限制（沿用原生体验）。",
};

function openEditor(i: number) {
  emit("openEditor", i);
}

async function openSettingsFolder() {
  try {
    await ProxyService.OpenSettingsFolder();
  } catch (e: any) {
    alert(e?.message ?? String(e));
  }
}

function openRepo() {
  Browser.OpenURL(GITHUB_URL).catch((e: any) => {
    alert(`无法打开浏览器: ${e?.message ?? String(e)}`);
  });
}

function parseVersion(tag: string): [number, number, number] {
  const cleaned = tag.trim().replace(/^v/i, "").split(/[-+]/)[0] ?? "";
  const parts = cleaned.split(".").map((p: string) => parseInt(p, 10));
  return [parts[0] || 0, parts[1] || 0, parts[2] || 0];
}

function isNewer(remote: string, local: string): boolean {
  const r = parseVersion(remote);
  const l = parseVersion(local);
  for (let i = 0; i < 3; i++) {
    if (r[i] > l[i]) return true;
    if (r[i] < l[i]) return false;
  }
  return false;
}

async function checkForUpdates() {
  if (updateBusy.value) return;
  updateBusy.value = true;
  try {
    const resp = await fetch(
      `https://api.github.com/repos/${GITHUB_REPO}/releases/latest`,
      { headers: { Accept: "application/vnd.github+json" } },
    );
    if (resp.status === 404) {
      alert(`No releases published yet.\nYou are running v${APP_VERSION}.`);
      return;
    }
    if (!resp.ok) {
      throw new Error(`GitHub returned HTTP ${resp.status}`);
    }
    const data = (await resp.json()) as {
      tag_name?: string;
      name?: string;
      html_url?: string;
    };
    const tag = data.tag_name ?? "";
    const htmlURL = data.html_url ?? `${GITHUB_URL}/releases/latest`;
    if (!tag) {
      alert(`You are running v${APP_VERSION}. Could not read latest tag.`);
      return;
    }
    if (isNewer(tag, APP_VERSION)) {
      const open = confirm(
        `A new version is available.\n\nInstalled: v${APP_VERSION}\nLatest: ${tag}\n\nOpen the release page in your browser?`,
      );
      if (open) {
        Browser.OpenURL(htmlURL).catch((e: any) => {
          alert(`Could not open browser: ${e?.message ?? String(e)}`);
        });
      }
    } else {
      alert(`You are on the latest version (v${APP_VERSION}).`);
    }
  } catch (e: any) {
    alert(`Update check failed: ${e?.message ?? String(e)}`);
  } finally {
    updateBusy.value = false;
  }
}

const emit = defineEmits<{
  (e: "openEditor", index: number): void;
}>();
</script>

<template>
  <div class="page">
    <!-- Model list -->
    <div class="overview-section-head">
      <h3>模型</h3>
      <button class="btn btn-primary btn-sm" @click="openEditor(-1)">
        + 添加模型
      </button>
    </div>
    <div v-if="!store.cfg.modelAdapters.length" class="empty-mini">
      暂无已配置的模型。
    </div>
    <div v-else class="overview-models">
      <article
        v-for="(a, i) in store.cfg.modelAdapters"
        :key="i"
        class="ov-model"
        @click="openEditor(i)"
      >
        <component
          :is="a.type === 'anthropic' ? AnthropicMark : OpenAIMark"
          class="ov-logo"
        />
        <div class="ov-info">
          <div class="ov-name">{{ a.displayName || "未命名" }}</div>
          <div class="ov-id mono">{{ a.modelID || "—" }}</div>
        </div>
        <span
          :class="[
            'mc-status',
            a.lastTestResult === 'ok'
              ? 'ms-ok'
              : a.lastTestResult
                ? 'ms-err'
                : 'ms-none',
          ]"
        >
          <span class="mc-status-dot" />
          {{
            a.lastTestResult === "ok" ? "OK" : a.lastTestResult ? "Err" : "—"
          }}
        </span>
      </article>
    </div>

    <!-- Settings rows -->
    <div class="card" style="margin-top: 16px">
      <div class="row">
        <div class="row-text">
          <div class="row-title">默认活动模型</div>
          <div class="row-desc">
            Controls which model new chats and unqualified requests use by default.
          </div>
          <div class="special-model-grid">
            <label class="special-model-field">
              <span>New chats</span>
              <select v-model="store.cfg.activeModelID" @change="store.persistConfig">
                <option value="">First configured model</option>
                <option v-for="opt in store.modelOptions" :key="`active-${opt.value}`" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
            </label>
          </div>
        </div>
      </div>
      <div class="hr" />
      <div class="row">
        <div class="row-text">
          <div class="row-title">Specialized model routing</div>
          <div class="row-desc">
            Choose dedicated models for commit message generation and code review.
          </div>
          <div class="special-model-grid">
            <label class="special-model-field">
              <span>Commit generator</span>
              <select v-model="store.cfg.commitModelID" @change="store.persistConfig">
                <option value="">Default model</option>
                <option v-for="opt in store.modelOptions" :key="`commit-${opt.value}`" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
            </label>
            <label class="special-model-field">
              <span>Code review</span>
              <select v-model="store.cfg.reviewModelID" @change="store.persistConfig">
                <option value="">Default model</option>
                <option v-for="opt in store.modelOptions" :key="`review-${opt.value}`" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
            </label>
          </div>
        </div>
      </div>
      <div class="hr" />
      <div class="row">
        <div class="row-text">
          <div class="row-title">
            Trust local CA
            <span
              :class="[
                'row-chip',
                store.state.caInstalled ? 'chip-ok' : 'chip-warn',
              ]"
            >
              {{ store.state.caInstalled ? "Installed" : "Not installed" }}
            </span>
          </div>
          <div class="row-desc">
            Adds cursor-byok's CA to the current-user trusted store so
            Cursor can verify TLS on intercepted connections.
          </div>
          <div class="row-subdesc">
            Mode: {{ store.state.caInstallMode || "manual" }}
          </div>
          <div v-if="store.state.caWarning" class="ca-warning">
            {{ store.state.caWarning }}
          </div>
          <code class="row-path">SHA-256 {{ store.shortFP }}</code>
        </div>
        <div class="row-actions">
          <button
            :class="['btn', store.state.caInstalled ? 'btn-ghost' : 'btn-primary']"
            :disabled="store.caBusy"
            @click="store.toggleCAInstall"
          >
            {{ store.state.caInstalled ? "Uninstall CA" : "Install CA" }}
          </button>
        </div>
      </div>
      <div class="hr" />
      <div class="row">
        <div class="row-text">
          <div class="row-title">
            Agent loop limits
            <span class="row-chip chip-info">0 = native</span>
          </div>
          <div class="row-desc">
            Cap the agent tool-call loop. 0 means no limit — Cursor's native
            behaviour where the client controls when to stop.
          </div>
          <div class="special-model-grid">
            <label class="special-model-field">
              <span>Max rounds</span>
              <input
                v-model.number="store.cfg.maxLoopRounds"
                type="number"
                min="0"
                max="200"
                placeholder="0"
                @change="store.persistConfig"
              />
            </label>
            <label class="special-model-field">
              <span>Max duration (min)</span>
              <input
                v-model.number="store.cfg.maxTurnDurationMin"
                type="number"
                min="0"
                max="120"
                placeholder="0"
                @change="store.persistConfig"
              />
            </label>
          </div>
        </div>
      </div>
      <div class="hr" />
      <div class="row">
        <div class="row-text">
          <div class="row-title">Settings folder</div>
          <div class="row-desc">Your config and CA live here.</div>
          <code class="row-path">{{
            store.state.caPath?.replace(/\\ca\\ca\.crt$/, "") || ""
          }}</code>
        </div>
        <div class="row-actions">
          <button class="btn btn-ghost" @click="openSettingsFolder">
            Open folder
          </button>
        </div>
      </div>
    </div>

    <div v-if="store.state.lastError" class="error-banner">
      {{ store.state.lastError }}
      <button class="error-dismiss" @click="store.clearError()" title="Dismiss">✕</button>
    </div>

    <div class="footer">
      <span>v{{ APP_VERSION }}</span>
      <span class="sep">·</span>
      <span>{{ OS_LABEL }}</span>
      <span class="footer-spacer" />
      <button class="link-btn" @click="openRepo">GitHub</button>
      <button
        class="link-btn"
        @click="checkForUpdates"
        :disabled="updateBusy"
      >
        {{ updateBusy ? "Checking…" : "Check for updates" }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.page {
  flex: 1;
  padding: 20px 24px 40px;
  max-width: 1040px;
  width: 100%;
  margin: 0 auto;
  box-sizing: border-box;
}

.overview-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.overview-section-head h3 {
  font-size: 14px;
  font-weight: 600;
  color: #e4e4e7;
  margin: 0;
}

.overview-models {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.ov-model {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: #101012;
  border: 1px solid #1f1f22;
  border-radius: 10px;
  cursor: pointer;
  transition: border-color 0.15s;
}
.ov-model:hover {
  border-color: #3f3f46;
}
.ov-logo {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  opacity: 0.7;
}
.ov-info {
  flex: 1;
  min-width: 0;
}
.ov-name {
  font-size: 13px;
  font-weight: 500;
  color: #fafafa;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ov-id {
  font-size: 11px;
  color: #71717a;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.footer {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 4px 0;
  color: #52525b;
  font-size: 11px;
}
.footer-spacer {
  flex: 1;
}
</style>
