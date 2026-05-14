<script setup lang="ts">
import { ref } from "vue";
import { useProxyStore } from "../stores/proxy";
import type { Provider } from "../types";
import OpenAIMark from "./logos/OpenAIMark.vue";
import AnthropicMark from "./logos/AnthropicMark.vue";

const store = useProxyStore();

const testingIndex = ref(-1);

async function testAdapter(i: number) {
  testingIndex.value = i;
  try {
    await store.testAdapter(i);
  } finally {
    testingIndex.value = -1;
  }
}

function shortHost(url: string) {
  if (!url) return "—";
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}

function obscure(key: string) {
  if (!key) return "—";
  if (key.length <= 8) return "••••";
  return key.slice(0, 4) + "••••" + key.slice(-4);
}

const emit = defineEmits<{
  (e: "openEditor", index: number): void;
}>();

function openEditor(i: number) {
  emit("openEditor", i);
}
</script>

<template>
  <div class="page">
    <div class="provider-bar">
      <button
        :class="['prov', store.providerTab === 'openai' ? 'prov-openai' : '']"
        @click="store.providerTab = 'openai'"
      >
        <OpenAIMark class="prov-logo" />
        <span>OpenAI</span>
        <span class="prov-count">{{ store.adapterCountByType('openai') }}</span>
      </button>
      <button
        :class="['prov', store.providerTab === 'anthropic' ? 'prov-anthropic' : '']"
        @click="store.providerTab = 'anthropic'"
      >
        <AnthropicMark class="prov-logo" />
        <span>Anthropic</span>
        <span class="prov-count">{{ store.adapterCountByType('anthropic') }}</span>
      </button>

      <span class="tab-spacer" />

      <button
        class="btn btn-ghost"
        @click="store.testAll"
        :disabled="!store.filteredAdapters.length"
      >
        Test all
      </button>
      <button class="btn btn-primary" @click="openEditor(-1)">
        + 添加模型
      </button>
    </div>

    <div v-if="!store.filteredAdapters.length" class="empty">
      <div class="empty-icon">
        <OpenAIMark v-if="store.providerTab === 'openai'" />
        <AnthropicMark v-else />
      </div>
      <div class="empty-title">
        No {{ store.providerTab === "openai" ? "OpenAI" : "Anthropic" }} models yet
      </div>
      <div class="empty-desc">
        Add a model to route BYOK requests through your own API key.
      </div>
      <button class="btn btn-primary" @click="openEditor(-1)">
        + Add your first model
      </button>
    </div>

    <div class="model-grid">
      <article
        v-for="{ a, i } in store.filteredAdapters"
        :key="i"
        class="model-card"
      >
        <header class="mc-head">
          <div class="mc-title">
            <component
              :is="a.type === 'anthropic' ? AnthropicMark : OpenAIMark"
              class="mc-logo"
            />
            <div>
              <div class="mc-name">{{ a.displayName || "未命名" }}</div>
              <div class="mc-id mono">{{ a.modelID || "—" }}</div>
            </div>
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
              a.lastTestResult === "ok"
                ? "Healthy"
                : a.lastTestResult
                  ? "Error"
                  : "Untested"
            }}
          </span>
        </header>

        <dl class="mc-grid">
          <div>
            <dt>Host</dt>
            <dd class="mono">{{ shortHost(a.baseURL) }}</dd>
          </div>
          <div>
            <dt>API key</dt>
            <dd class="mono">{{ obscure(a.apiKey) }}</dd>
          </div>
        </dl>

        <footer class="mc-actions">
          <button
            class="chip"
            @click="testAdapter(i)"
            :disabled="testingIndex === i"
          >
            {{ testingIndex === i ? "Testing..." : "Test" }}
          </button>
          <button class="chip" @click="openEditor(i)">Edit</button>
          <button class="chip" @click="store.duplicate(i)">Duplicate</button>
          <span class="tab-spacer" />
          <button class="chip chip-danger" @click="store.removeAdapter(i)">
            Delete
          </button>
        </footer>
      </article>
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

.model-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
}
.model-card {
  background: #101012;
  border: 1px solid #1f1f22;
  border-radius: 12px;
  padding: 14px 16px 12px;
}
.mc-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
  gap: 10px;
}
.mc-title {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.mc-logo {
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  color: #a1a1aa;
}
.mc-name {
  color: #fafafa;
  font-size: 14px;
  font-weight: 500;
}
.mc-id {
  color: #71717a;
  font-size: 11px;
  margin-top: 2px;
}

.mc-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin: 0 0 12px;
}
.mc-grid > div {
  background: #0a0a0c;
  border: 1px solid #1f1f22;
  border-radius: 8px;
  padding: 6px 10px;
  min-width: 0;
}
.mc-grid dt {
  font-size: 10px;
  color: #71717a;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 2px;
}
.mc-grid dd {
  margin: 0;
  color: #fafafa;
  font-size: 12.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mc-actions {
  display: flex;
  gap: 4px;
  align-items: center;
  padding-top: 4px;
  border-top: 1px solid #1f1f22;
  margin-top: 4px;
}
</style>
