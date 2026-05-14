// Provider presets: default URLs, model IDs, and display names for known providers.
import type { Provider } from "./types";

export interface ProviderPreset {
  id: Provider;
  label: string;
  defaultBaseURL: string;
  defaultModel: string;
  openAICompatible: boolean;
}

export const PROVIDER_PRESETS: ProviderPreset[] = [
  {
    id: "openai",
    label: "OpenAI",
    defaultBaseURL: "https://api.openai.com/v1",
    defaultModel: "gpt-4o",
    openAICompatible: true,
  },
  {
    id: "anthropic",
    label: "Anthropic",
    defaultBaseURL: "https://api.anthropic.com",
    defaultModel: "claude-sonnet-4-5",
    openAICompatible: false,
  },
  {
    id: "gemini",
    label: "Google Gemini",
    defaultBaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
    defaultModel: "gemini-2.5-pro",
    openAICompatible: true,
  },
  {
    id: "deepseek",
    label: "DeepSeek",
    defaultBaseURL: "https://api.deepseek.com/v1",
    defaultModel: "deepseek-chat",
    openAICompatible: true,
  },
  {
    id: "ollama",
    label: "Ollama (local)",
    defaultBaseURL: "http://localhost:11434/v1",
    defaultModel: "llama3",
    openAICompatible: true,
  },
  {
    id: "mistral",
    label: "Mistral",
    defaultBaseURL: "https://api.mistral.ai/v1",
    defaultModel: "mistral-large-latest",
    openAICompatible: true,
  },
  {
    id: "custom",
    label: "Custom (OpenAI-compatible)",
    defaultBaseURL: "",
    defaultModel: "",
    openAICompatible: true,
  },
];

export function getPreset(id: Provider): ProviderPreset {
  return PROVIDER_PRESETS.find((p) => p.id === id) ?? PROVIDER_PRESETS[0];
}
