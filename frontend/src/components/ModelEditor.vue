<script setup lang="ts">
import { ref, computed } from "vue";
import { useProxyStore } from "../stores/proxy";
import type { ModelAdapter, Provider } from "../types";
import { PROVIDER_PRESETS, getPreset } from "../providers";
import OpenAIMark from "./logos/OpenAIMark.vue";
import AnthropicMark from "./logos/AnthropicMark.vue";

const store = useProxyStore();

const HELP = {
  displayName:
    "在选择器中显示的标签。自由填写，不会发送给提供商。",
  modelID:
    "发送给上游提供商的规范模型标识符（如 gpt-4o、claude-sonnet-4-5）。",
  apiKey:
    "提供商 API 密钥。本地存储于 config.json 中，不会在 BYOK 调用之外传输。",
  baseURL:
    "提供商端点。可覆盖为代理/反向代理/Azure OpenAI 等。",
  contextWindow:
    "发送给提供商的最大输入词元数。留空使用提供商默认值。",
  reasoningEffort:
    "OpenAI 推理模型（o1、o3、gpt-5 系列）的推理预算提示。",
  fastMode:
    "使用优先服务层级以获得更快响应（OpenAI）。每词元费用更高。",
  maxOutput:
    "每次响应的输出词元上限。留空使用提供商默认值。",
  thinkingBudget:
    "Anthropic 扩展思考词元预算。仅适用于支持推理的模型。",
  retryCount:
    "上游请求失败时的最大重试次数。留空使用默认值 2（推荐）。适用于 429/5xx 等可重试错误。",
  retryInterval:
    "重试间隔基础值（毫秒）。实际间隔按指数退避递增，上限 60 秒。留空使用默认值 1000。",
  timeout:
    "单次上游请求超时时间（毫秒）。留空使用默认值 300000（5 分钟）。",
  notes: "私人备注。不会发送到任何地方。",
};

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
        <button class="link-btn" @click="cancelEditor">← Models</button>
        <h2 class="editor-title">
          {{ props.editorIndex === -1 ? "New model" : "Edit model" }}
          <span class="editor-sub">{{
            props.editorModel?.displayName ? "— " + props.editorModel.displayName : ""
          }}</span>
        </h2>
      </div>
      <div class="row-actions">
        <button class="btn btn-ghost" @click="cancelEditor">Cancel</button>
        <button class="btn btn-ghost" @click="saveEditor(true)">
          Save and test
        </button>
        <button class="btn btn-primary" @click="saveEditor(false)">
          Save
        </button>
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
        :class="[
          'prov',
          props.editorProvider === 'anthropic' ? 'prov-anthropic' : '',
        ]"
        @click="emit('update:editorProvider', 'anthropic')"
      >
        <AnthropicMark class="prov-logo" /> Anthropic
      </button>
    </div>

    <section class="card form-section">
      <div class="section-head">
        <h3>Identity</h3>
        <p>Shown in the picker and mapped to the upstream provider.</p>
      </div>
      <div class="form-grid">
        <div class="field">
          <label
            >Display name
            <span class="info" :data-tip="HELP.displayName">i</span></label
          >
          <input
            :value="props.editorModel?.displayName"
            @input="emit('update:editorModel', { ...props.editorModel!, displayName: ($event.target as HTMLInputElement).value })"
            placeholder="GPT-4o (work)"
          />
        </div>
        <div class="field">
          <label
            >Model ID
            <span class="info" :data-tip="HELP.modelID">i</span></label
          >
          <input
            :value="props.editorModel?.modelID"
            @input="emit('update:editorModel', { ...props.editorModel!, modelID: ($event.target as HTMLInputElement).value })"
            :placeholder="
              props.editorProvider === 'openai' ? 'gpt-4o' : 'claude-sonnet-4-5'
            "
          />
        </div>
      </div>
    </section>

    <section class="card form-section">
      <div class="section-head">
        <h3>Endpoint & credentials</h3>
        <p>Never leaves this machine — stored in <code>config.json</code>.</p>
      </div>
      <div class="form-grid">
        <div class="field">
          <label
            >API key
            <span class="info" :data-tip="HELP.apiKey">i</span></label
          >
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
              :title="props.showApiKey ? 'Hide key' : 'Show key'"
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
                <path
                  d="M3 3l18 18M10.58 10.58a2 2 0 0 0 2.83 2.83M9.36 5.64A10.94 10.94 0 0 1 12 5c7 0 11 7 11 7a17.07 17.07 0 0 1-3.3 4.38M6.1 6.1C3.4 7.8 1 12 1 12s4 7 11 7a10.78 10.78 0 0 0 5-1.23"
                />
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
          <label
            >Base URL
            <span class="info" :data-tip="HELP.baseURL">i</span></label
          >
          <input
            :value="props.editorModel?.baseURL"
            @input="emit('update:editorModel', { ...props.editorModel!, baseURL: ($event.target as HTMLInputElement).value })"
            :placeholder="
              props.editorProvider === 'openai'
                ? 'https://api.openai.com/v1'
                : 'https://api.anthropic.com'
            "
          />
        </div>
      </div>
    </section>

    <section class="card form-section">
      <div class="section-head">
        <h3>Advanced</h3>
        <p>All optional — leave blank to use provider defaults.</p>
      </div>
      <div class="form-grid">
        <div class="field">
          <label
            >Context window
            <span class="info" :data-tip="HELP.contextWindow">i</span></label
          >
          <input
            :value="props.editorModel?.contextWindow"
            @input="emit('update:editorModel', { ...props.editorModel!, contextWindow: ($event.target as HTMLInputElement).value })"
            placeholder="200000"
          />
        </div>
        <div v-if="props.editorProvider === 'openai'" class="field">
          <label
            >Reasoning effort
            <span class="info" :data-tip="HELP.reasoningEffort"
              >i</span
            ></label
          >
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
              @change="
                emit('update:editorModel', { ...props.editorModel!, serviceTier: ($event.target as HTMLInputElement).checked ? 'priority' : '' })
              "
            />
            Fast mode <span class="info" :data-tip="HELP.fastMode">i</span>
          </label>
        </div>
        <div class="field">
          <label
            >Max output tokens
            <span class="info" :data-tip="HELP.maxOutput">i</span></label
          >
          <input
            :value="props.editorModel?.maxOutputTokens"
            @input="emit('update:editorModel', { ...props.editorModel!, maxOutputTokens: Number(($event.target as HTMLInputElement).value) || 0 })"
            type="number"
            min="0"
            placeholder="65536"
          />
        </div>
        <div v-if="props.editorProvider === 'anthropic'" class="field">
          <label
            >Thinking budget
            <span class="info" :data-tip="HELP.thinkingBudget">i</span></label
          >
          <input
            :value="props.editorModel?.thinkingBudget"
            @input="emit('update:editorModel', { ...props.editorModel!, thinkingBudget: Number(($event.target as HTMLInputElement).value) || 0 })"
            type="number"
            min="0"
            placeholder="16000"
          />
        </div>
        <div class="field">
          <label
            >Retry count
            <span class="info" :data-tip="HELP.retryCount">i</span></label
          >
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
          <label
            >Retry interval (ms)
            <span class="info" :data-tip="HELP.retryInterval">i</span></label
          >
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
          <label
            >Timeout (ms)
            <span class="info" :data-tip="HELP.timeout">i</span></label
          >
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
          <label
            >Notes <span class="info" :data-tip="HELP.notes">i</span></label
          >
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
        <h3>Test result</h3>
        <p>Last probe against this adapter.</p>
      </div>
      <div v-if="!props.editorModel?.lastTestResult" class="test-state test-none">
        <span class="mc-status-dot" /> No test run yet — use
        <b>Save and test</b>.
      </div>
      <div
        v-else-if="props.editorModel?.lastTestResult === 'ok'"
        class="test-state test-ok"
      >
        <span class="mc-status-dot" /> Healthy — adapter responded
        successfully.
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