<script setup lang="ts">
import { Window, Browser } from "@wailsio/runtime";
import { useProxyStore } from "../stores/proxy";

const store = useProxyStore();

const GITHUB_REPO = "lijinhan/CursorBridge";
const GITHUB_URL = `https://github.com/${GITHUB_REPO}`;

function winMinimise() {
  Window.Minimise();
}
function winHide() {
  Window.Hide();
}
function openRepo() {
  Browser.OpenURL(GITHUB_URL).catch((e: any) => {
    alert(`无法打开浏览器: ${e?.message ?? String(e)}`);
  });
}
</script>

<template>
  <header class="topbar">
    <div class="brand">
      <div class="logo-mark">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none">
          <path
            d="M4 7l8-4 8 4-8 4-8-4z"
            stroke="#22c55e"
            stroke-width="1.5"
            stroke-linejoin="round"
          />
          <path
            d="M4 12l8 4 8-4M4 17l8 4 8-4"
            stroke="#52525b"
            stroke-width="1.5"
            stroke-linejoin="round"
          />
        </svg>
      </div>
      <div>
        <div class="brand-name">cursor-byok</div>
        <div class="brand-sub">本地 MITM · BYOK 网关</div>
      </div>
    </div>

    <div class="topbar-right">
      <div :class="['status-pill', store.state.running ? 'pill-on' : 'pill-off']">
        <span class="dot"></span>
        {{ store.state.running ? `运行中 · ${store.state.listenAddr}` : "已停止" }}
      </div>
      <button
        :class="['btn', store.state.running ? 'btn-ghost' : 'btn-primary']"
        :disabled="store.busy"
        @click="store.toggleService"
      >
        {{ store.state.running ? "停止" : "启动服务" }}
      </button>

      <div class="win-controls">
        <button
          class="win-btn"
          @click="openRepo"
          title="Open GitHub repository"
          aria-label="Open GitHub repository"
        >
          <svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true">
            <path
              fill="currentColor"
              d="M8 0C3.58 0 0 3.58 0 8a8 8 0 0 0 5.47 7.59c.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.4 7.4 0 0 1 2-.27c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"
            />
          </svg>
        </button>
        <button class="win-btn" @click="winMinimise" title="Minimise">
          <svg viewBox="0 0 12 12" width="10" height="10">
            <line
              x1="2"
              y1="6"
              x2="10"
              y2="6"
              stroke="currentColor"
              stroke-width="1.3"
              stroke-linecap="round"
            />
          </svg>
        </button>
        <button
          class="win-btn win-close"
          @click="winHide"
          title="Hide to tray"
        >
          <svg viewBox="0 0 12 12" width="10" height="10">
            <path
              d="M2 2 L10 10 M10 2 L2 10"
              stroke="currentColor"
              stroke-width="1.3"
              stroke-linecap="round"
            />
          </svg>
        </button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px 10px 18px;
  background: rgba(9, 9, 11, 0.85);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid #1f1f22;
  --wails-draggable: drag;
}
.topbar button,
.topbar .status-pill,
.topbar input,
.topbar .win-controls {
  --wails-draggable: no-drag;
}

.win-controls {
  display: inline-flex;
  margin-left: 4px;
}
.win-btn {
  background: transparent;
  border: 0;
  color: #71717a;
  width: 34px;
  height: 28px;
  border-radius: 6px;
  display: grid;
  place-items: center;
  cursor: pointer;
  font-family: inherit;
  transition:
    background 0.12s,
    color 0.12s;
}
.win-btn:hover {
  background: #18181b;
  color: #fafafa;
}
.win-close:hover {
  background: #7f1d1d;
  color: #fff;
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
}
.logo-mark {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  background: #0b0b0d;
  border: 1px solid #1f1f22;
  border-radius: 8px;
}
.brand-name {
  font-size: 14px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: #fafafa;
}
.brand-sub {
  font-size: 11px;
  color: #71717a;
  margin-top: 1px;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
}
.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 5px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  border: 1px solid transparent;
}
.status-pill .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}
.pill-on {
  color: #4ade80;
  background: rgba(34, 197, 94, 0.08);
  border-color: rgba(34, 197, 94, 0.25);
}
.pill-on .dot {
  background: #22c55e;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.25);
  animation: pulse 1.8s ease-in-out infinite;
}
.pill-off {
  color: #a1a1aa;
  background: #18181b;
  border-color: #27272a;
}
.pill-off .dot {
  background: #52525b;
}
@keyframes pulse {
  50% {
    box-shadow: 0 0 0 6px rgba(34, 197, 94, 0);
  }
}
</style>
