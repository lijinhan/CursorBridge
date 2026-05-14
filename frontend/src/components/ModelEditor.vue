<script setup lang="ts">
import { ref, computed } from "vue";
import { useProxyStore } from "../stores/proxy";
import type { ModelAdapter, Provider } from "../types";
import { t } from "../i18n";
import OpenAIMark from "./logos/OpenAIMark.vue";
import AnthropicMark from "./logos/AnthropicMark.vue";

const store = useProxyStore();

const props = defineProps<{
  editorIndex: number;
  editorModel: ModelAdapter | null;
  editorProvider: Provider;
  showApiKey: boolean;
  editorTestRunning: boolean;
}>();

const emit = defineEmits<{
  (e: "cancel"): void;
  (e: "save", runTest: boolean): void;
  (e: "update:editorProvider", value: Provider): void;
  (e: "update:showApiKey", value: boolean): void;
  (e: "update:editorModel", value: ModelAdapter): void;
}>();

function cancelEditor() {
  emit("cancel");
}

function saveEditor(runTest: boolean) {
  emit("save", runTest);
}
</script>

<template>
  <div class="page">
    <div class="editor-head">
      <div>
        <button class="link-btn" @click="cancelEditor">{{ t('editor.back') }}</button>
        <h2 class="editor-title">
          {{ props.editorIndex === -1 ? t('editor.new') : t('editor.edit') }}
          <span class="editor-sub">{{
            props.editorModel?.displayName ? "— " + props.editorModel.displayName : ""
          }}</span>
        </h2>
      </div>
      <div class="row-actions">
        <button class="btn btn-ghost" @click="cancelEditor">{{ t('editor.cancel') }}</button>
        <button class="btn btn-ghost" @click="saveEditor(true)">{{ t('editor.saveAndTest') }}</button>
        <button class="btn btn-primary" @click="saveEditor(false)">{{ t('editor.save') }}</button>
      </div>
    </div>

    <div class="provider-bar slim">
      <button
        :class="['prov', props.editorProvider === 'openai' ? 'prov-openai' : '']"
        @click="emit('update:editorProvider', 'openai')"
      >
        <OpenAIMark class="prov-logo" /> OpenAI
      </button>
      <button
        :class="['prov', props.editorProvider === 'anthropic' ? 'prov-anthropic' : '']"
        @click="emit('update:editorProvider', 'anthropic')"
      >
        <AnthropicMark class="prov-logo" /> Anthropic
      </button>
    </div>

    <section class="card form-section">
      <div class="section-head">
        <h3>{{ t('editor.identity') }}</h3>
        <p>{{ t('editor.identityDesc') }}</p>
      </div>
      <div class="form-grid">
        <div class="field">
          <label>{{ t('editor.displayName') }}
            <span class="info" :data-tip="t('help.displayName')">i</span></label>
          <input
            :value="props.editorModel?.displayName"
            @input="emit('update:editorModel', { ...props.editorModel!, displayName: ($event.target as HTMLInputElement).value })"
            placeholder="GPT-4o (work)"
          />
        </div>
        <div class="field">
          <label>{{ t('editor.modelID') }}
            <span class="info" :data-tip="t('help.modelID')">i</span></label>
          <input
            :value="props.editorModel?.modelID"
            @input="emit('update:editorModel', { ...props.editorModel!, modelID: ($event.target as HTMLInputElement).value })"
            :placeholder="props.editorProvider === 'openai' ? 'gpt-4o' : 'claude-sonnet-4-5'"
          />
        </div>
      </div>
    </section>

    <section class="card form-section">
      <div class="section-head">
        <h3>{{ t('editor.endpoint') }}</h3>
        <p>{{ t('editor.endpointDesc') }}</p>
      </div>
      <div class="form-grid">
        <div class="field">
          <label>{{ t('editor.apiKey') }}
            <span class="info" :data-tip="t('help.apiKey')">i</span></label>
          <div class="input-with-action">
            <input
              :type="props.showApiKey ? 'text' : 'password'"
              :value="props.editorModel?.apiKey"
              @input="emit('update:editorModel', { ...props.editorModel!, apiKey: ($event.target as HTMLInputElement).value })"
              placeholder="sk-…"
            />
            <button
              class="eye"
              @click="emit('update:showApiKey', !props.showApiKey)"
              type="button"
              :title="props.showApiKey ? t('editor.hideKey') : t('editor.showKey')"
            >
              <svg
                v-if="props.showApiKey"
                viewBox="0 0 24 24"
                width="14"
                height="14"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
              >
                <path d="M3 3l18 18M10.58 10.58a2 2 0 0 0 2.83 2.83M9.36 5.64A10.94 10.94 0 0 1 12 5c7 0 11 7 11 7a17.07 17.07 0 0 1-3.3 4.38M6.1 6.1C3.4 7.8 1 12 1 12s4 7 11 7a10.78 10.78 0 0 0 5-1.23" />
              </svg>
              <svg
                v-else
                viewBox="0 0 24 24"
                width="14"
                height="14"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
              >
                <path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7S1 12 1 12z" />
                <circle cx="12" cy="12" r="3" />
              </svg>
            </button>
          </div>
        </div>
        <div class="field">
          <label>{{ t('editor.baseURL') }}
            <span class="info" :data-tip="t('help.baseURL')">i</span></label>
          <input
            :value="props.editorModel?.baseURL"
            @input="emit('update:editorModel', { ...props.editorModel!, baseURL: ($event.target as HTMLInputElement).value })"
            :placeholder="props.editorProvider === 'openai' ? 'https://api.openai.com/v1' : 'https://api.anthropic.com'"
          />
        </div>
      </div>
    </section>

    <section class="card form-section">
      <div class="section-head">
        <h3>{{ t('editor.advanced') }}</h3>
        <p>{{ t('editor.advancedDesc') }}</p>
      </div>
      <div class="form-grid">
        <div class="field">
          <label>{{ t('editor.contextWindow') }}
            <span class="info" :data-tip="t('help.contextWindow')">i</span></label>
          <input
            :value="props.editorModel?.contextWindow"
            @input="emit('update:editorModel', { ...props.editorModel!, contextWindow: ($event.target as HTMLInputElement).value })"
            placeholder="200000"
          />
        </div>
        <div v-if="props.editorProvider === 'openai'" class="field">
          <label>{{ t('editor.reasoningEffort') }}
            <span class="info" :data-tip="t('help.reasoningEffort')">i</span></label>
          <select
            :value="props.editorModel?.reasoningEffort"
            @change="emit('update:editorModel', { ...props.editorModel!, reasoningEffort: ($event.target as HTMLSelectElement).value })"
          >
            <option value="none">None</option>
            <option value="low">Low</option>
            <option value="medium">Medium</option>
            <option value="high">High</option>
            <option value="xhigh">XHigh</option>
          </select>
          <label class="fast-toggle">
            <input
              type="checkbox"
              :checked="props.editorModel?.serviceTier === 'priority'"
              @change="emit('update:editorModel', { ...props.editorModel!, serviceTier: ($event.target as HTMLInputElement).checked ? 'priority' : '' })"
            />
            {{ t('editor.fastMode') }} <span class="info" :data-tip="t('help.fastMode')">i</span>
          </label>
        </div>
        <div class="field">
          <label>{{ t('editor.maxOutput') }}
            <span class="info" :data-tip="t('help.maxOutput')">i</span></label>
          <input
            :value="props.editorModel?.maxOutputTokens"
            @input="emit('update:editorModel', { ...props.editorModel!, maxOutputTokens: Number(($event.target as HTMLInputElement).value) || 0 })"
            type="number"
            min="0"
            placeholder="65536"
          />
        </div>
        <div v-if="props.editorProvider === 'anthropic'" class="field">
          <label>{{ t('editor.thinkingBudget') }}
            <span class="info" :data-tip="t('help.thinkingBudget')">i</span></label>
          <input
            :value="props.editorModel?.thinkingBudget"
            @input="emit('update:editorModel', { ...props.editorModel!, thinkingBudget: Number(($event.target as HTMLInputElement).value) || 0 })"
            type="number"
            min="0"
            placeholder="16000"
          />
        </div>
        <div class="field">
          <label>{{ t('editor.retryCount') }}
            <span class="info" :data-tip="t('help.retryCount')">i</span></label>
          <input
            :value="props.editorModel?.retryCount"
            @input="emit('update:editorModel', { ...props.editorModel!, retryCount: Number(($event.target as HTMLInputElement).value) || 0 })"
            type="number"
            min="0"
            max="10"
            placeholder="0"
          />
        </div>
        <div class="field">
          <label>{{ t('editor.retryInterval') }}
            <span class="info" :data-tip="t('help.retryInterval')">i</span></label>
          <input
            :value="props.editorModel?.retryInterval"
            @input="emit('update:editorModel', { ...props.editorModel!, retryInterval: Number(($event.target as HTMLInputElement).value) || 0 })"
            type="number"
            min="100"
            max="30000"
            placeholder="1000"
          />
        </div>
        <div class="field">
          <label>{{ t('editor.timeout') }}
            <span class="info" :data-tip="t('help.timeout')">i</span></label>
          <input
            :value="props.editorModel?.timeout"
            @input="emit('update:editorModel', { ...props.editorModel!, timeout: Number(($event.target as HTMLInputElement).value) || 0 })"
            type="number"
            min="1000"
            max="600000"
            placeholder="300000"
          />
        </div>
        <div class="field field-full">
          <label>{{ t('editor.notes') }} <span class="info" :data-tip="t('help.notes')">i</span></label>
          <textarea
            :value="props.editorModel?.notes"
            @input="emit('update:editorModel', { ...props.editorModel!, notes: ($event.target as HTMLTextAreaElement).value })"
            rows="3"
            placeholder="Optional notes — only visible to you."
          />
        </div>
      </div>
    </section>

    <section class="card form-section test-section">
      <div class="section-head">
        <h3>{{ t('editor.testResult') }}</h3>
        <p>{{ t('editor.testResultDesc') }}</p>
      </div>
      <div v-if="!props.editorModel?.lastTestResult" class="test-state test-none">
        <span class="mc-status-dot" /> {{ t('editor.noTest') }}
      </div>
      <div v-else-if="props.editorModel?.lastTestResult === 'ok'" class="test-state test-ok">
        <span class="mc-status-dot" /> {{ t('editor.healthy') }}
      </div>
      <div v-else class="test-state test-err">
        <span class="mc-status-dot" /> {{ props.editorModel?.lastTestResult }}
      </div>
    </section>
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

.editor-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}
.editor-title {
  margin: 4px 0 0;
  font-size: 20px;
  font-weight: 500;
  color: #fafafa;
  letter-spacing: -0.01em;
}
.editor-sub {
  color: #71717a;
  font-size: 14px;
  font-weight: 400;
  margin-left: 4px;
}

.form-section {
  padding: 18px 20px;
}

.test-section .test-state {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 13px;
}
.test-none {
  background: #0a0a0c;
  color: #71717a;
  border: 1px solid #1f1f22;
}
.test-ok {
  background: rgba(34, 197, 94, 0.08);
  color: #4ade80;
  border: 1px solid rgba(34, 197, 94, 0.25);
}
.test-err {
  background: rgba(239, 68, 68, 0.08);
  color: #f87171;
  border: 1px solid rgba(239, 68, 68, 0.25);
}
.test-section .mc-status-dot {
  width: 8px;
  height: 8px;
}
</style>