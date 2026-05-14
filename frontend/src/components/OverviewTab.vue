<script setup lang="ts">
import { computed, ref } from "vue";
import { ProxyService } from "../../bindings/cursorbridge/internal/bridge";
import { Browser, System } from "@wailsio/runtime";
import { useProxyStore } from "../stores/proxy";
import { t, locale, setLocale } from "../i18n";
import OpenAIMark from "./logos/OpenAIMark.vue";
import AnthropicMark from "./logos/AnthropicMark.vue";

const store = useProxyStore();

const APP_VERSION = "1.0.1";
const GITHUB_REPO = "lijinhan/CursorBridge";
const GITHUB_URL = `https://github.com/${GITHUB_REPO}`;

const OS_LABEL = (() => {
  try {
    if (System.IsWindows()) return t('settings.osWindows');
    if (System.IsMac()) return t('settings.osMac');
    if (System.IsLinux()) return t('settings.osLinux');
  } catch {
    /* runtime not ready (dev tools preview) — fall through */
  }
  return t('settings.osDesktop');
})();

const updateBusy = ref(false);

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
    alert(t('footer.cannotOpenBrowser', { error: e?.message ?? String(e) }));
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
          alert(t('footer.cannotOpenBrowser', { error: e?.message ?? String(e) }));
        });
      }
    } else {
      alert(t('footer.noUpdate'));
    }
  } catch (e: any) {
    alert(t('footer.updateError', { error: e?.message ?? String(e) }));
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
      <h3>{{ t('overview.models') }}</h3>
      <button class="btn btn-primary btn-sm" @click="openEditor(-1)">
        {{ t('overview.addModel') }}
      </button>
    </div>
    <div v-if="!store.cfg.modelAdapters.length" class="empty-mini">
      {{ t('overview.noModels') }}
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
          <div class="ov-name">{{ a.displayName || t('overview.unnamed') }}</div>
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
          <div class="row-title">{{ t('settings.activeModel') }}</div>
          <div class="row-desc">
            {{ t('settings.activeModelDesc') }}
          </div>
          <div class="special-model-grid">
            <label class="special-model-field">
              <span>{{ t('settings.newChats') }}</span>
              <select v-model="store.cfg.activeModelID" @change="store.persistConfig">
                <option value="">{{ t('settings.firstModel') }}</option>
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
          <div class="row-title">{{ t('settings.specializedRouting') }}</div>
          <div class="row-desc">
            {{ t('settings.activeModelDesc') }}
          </div>
          <div class="special-model-grid">
            <label class="special-model-field">
              <span>{{ t('settings.commitGen') }}</span>
              <select v-model="store.cfg.commitModelID" @change="store.persistConfig">
                <option value="">{{ t('settings.defaultModel') }}</option>
                <option v-for="opt in store.modelOptions" :key="`commit-${opt.value}`" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
            </label>
            <label class="special-model-field">
              <span>{{ t('settings.codeReview') }}</span>
              <select v-model="store.cfg.reviewModelID" @change="store.persistConfig">
                <option value="">{{ t('settings.defaultModel') }}</option>
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
            {{ t('settings.trustCA') }}
            <span
              :class="[
                'row-chip',
                store.state.caInstalled ? 'chip-ok' : 'chip-warn',
              ]"
            >
              {{ store.state.caInstalled ? t('settings.installed') : t('settings.notInstalled') }}
            </span>
          </div>
          <div class="row-desc">
            Adds CursorBridge's CA to the current-user trusted store so
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
            {{ store.state.caInstalled ? t('settings.uninstallCA') : t('settings.installCA') }}
          </button>
        </div>
      </div>
      <div class="hr" />
      <div class="row">
        <div class="row-text">
          <div class="row-title">
            {{ t('settings.loopLimits') }}
            <span class="row-chip chip-info">{{ t('settings.zeroNative') }}</span>
          </div>
          <div class="row-desc">
            {{ t('help.maxLoopRounds') }}
          </div>
          <div class="special-model-grid">
            <label class="special-model-field">
              <span>{{ t('settings.maxRounds') }}</span>
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
              <span>{{ t('settings.maxDuration') }}</span>
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
          <div class="row-title">{{ t('settings.settingsFolder') }}</div>
          <div class="row-desc">{{ t('settings.configDesc') }}</div>
          <code class="row-path">{{
            store.state.caPath?.replace(/\\ca\\ca\.crt$/, "") || ""
          }}</code>
        </div>
        <div class="row-actions">
          <button class="btn btn-ghost" @click="openSettingsFolder">
            {{ t('settings.openFolder') }}
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
      <select class="locale-select" :value="locale" @change="setLocale(($event.target as HTMLSelectElement).value)">
        <option value="zh-CN">中文</option>
        <option value="en">English</option>
      </select>
      <button class="link-btn" @click="openRepo">GitHub</button>
      <button
        class="link-btn"
        @click="checkForUpdates"
        :disabled="updateBusy"
      >
        {{ updateBusy ? t('footer.checking') : t('footer.checkUpdates') }}
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
.locale-select {
  background: #18181b;
  border: 1px solid #27272a;
  color: #a1a1aa;
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  cursor: pointer;
}
</style>