package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// 默认上下文窗口大小（token数），当未从模型配置获取时使用
	defaultContextTokenLimit = 128000

	// 保留的最近轮次数，这些轮次不会被压缩
	defaultRecentTurnsToKeep = 4

	// 触发压缩的上下文使用率阈值（0.0-1.0）
	compactionTriggerRatio = 0.85

	// 摘要生成时的系统提示词
	summarySystemPrompt = `You are a conversation summarizer. Your task is to create a concise but comprehensive summary of the preceding conversation turns. Preserve:
1. Key decisions and their rationale
2. Important code changes or file paths discussed
3. Unresolved issues or open questions
4. Tool execution results that are still relevant
Do NOT include pleasantries or filler. Write in a factual, reference-oriented style.`

	// 摘要生成的用户提示词前缀
	summaryUserPromptPrefix = "Summarize the following conversation turns. Focus on preserving actionable information, decisions, and context needed for continuing the task:\n\n"
)

// CompactionArchive 存储一次压缩产生的摘要及其元数据
type CompactionArchive struct {
	Summary      string    // LLM生成的摘要文本
	WindowTail   int       // 压缩后保留的最早轮次索引
	CreatedAt    time.Time // 压缩发生时间
	TurnCount    int       // 被压缩的轮次数
	CharSaved    int       // 被压缩节省的字符数
}

// CompactionState 管理一个会话的压缩状态
type CompactionState struct {
	mu             sync.RWMutex
	Archives       []CompactionArchive // 历史压缩归档
	CurrentSummary string              // 当前生效的累积摘要
	Compacting     bool                // 是否正在压缩中
	LastCompaction time.Time           // 上次压缩时间
}

// getCompactionState 获取或创建会话的压缩状态
func getCompactionState(conversationID string) *CompactionState {
	DefaultDeps.CompactionStatesMu.Lock()
	defer DefaultDeps.CompactionStatesMu.Unlock()
	if cs, ok := DefaultDeps.CompactionStates[conversationID]; ok {
		return cs
	}
	cs := &CompactionState{}
	DefaultDeps.CompactionStates[conversationID] = cs
	return cs
}


// compactTurns 执行上下文压缩：用LLM对旧轮次生成摘要，返回压缩后的消息列表
// recentTurnsToKeep: 保留的最近轮次数
// maxTokens: 模型上下文窗口token数
// stream: LLM provider stream函数
// baseURL, apiKey, model: BYOK provider配置
// opts: adapter选项
func compactTurns(ctx context.Context, turns []*ConvTurn, recentTurnsToKeep int, maxTokens int,
	stream providerStreamer, baseURL, apiKey, model string, opts AdapterOpts) (string, []*ConvTurn, error) {

	if len(turns) <= recentTurnsToKeep {
		return "", turns, nil
	}

	// 分割：旧轮次用于生成摘要，新轮次完整保留
	oldTurns := turns[:len(turns)-recentTurnsToKeep]
	recentTurns := turns[len(turns)-recentTurnsToKeep:]

	// 构建待摘要的文本
	var sb strings.Builder
	for i, t := range oldTurns {
		if i > 0 {
			sb.WriteString("\n---\n")
		}
		if t.User != "" {
			sb.WriteString("User: ")
			sb.WriteString(t.User)
			sb.WriteString("\n")
		}
		if t.Assistant != "" {
			sb.WriteString("Assistant: ")
			sb.WriteString(t.Assistant)
			sb.WriteString("\n")
		}
		for _, msg := range t.Messages {
			sb.WriteString(msg.Role)
			sb.WriteString(": ")
			sb.WriteString(msg.Content)
			sb.WriteString("\n")
		}
	}

	// 调用LLM生成摘要
	summary, err := generateSummary(ctx, sb.String(), stream, baseURL, apiKey, model, opts)
	if err != nil {
		return "", turns, fmt.Errorf("generate compaction summary: %w", err)
	}

	return summary, recentTurns, nil
}

// generateSummary 调用LLM生成对话摘要
func generateSummary(ctx context.Context, conversationText string,
	stream providerStreamer, baseURL, apiKey, model string, opts AdapterOpts) (string, error) {

	summaryOpts := opts
	summaryOpts.MaxOutputTokens = 2048
	summaryOpts.ReasoningEffort = ""
	summaryOpts.ServiceTier = ""

	messages := []openAIMessage{
		textMessage("system", summarySystemPrompt),
		textMessage("user", summaryUserPromptPrefix+conversationText),
	}

	result, err := collectSingleResponse(ctx, stream, baseURL, apiKey, model, summaryOpts, messages)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result), nil
}

// buildCompactionArchive 从压缩结果创建归档记录
func buildCompactionArchive(summary string, originalTurns []*ConvTurn, keptFromIndex int) CompactionArchive {
	originalChars := 0
	for _, t := range originalTurns[:keptFromIndex] {
		originalChars += len(t.User) + len(t.Assistant)
		for _, m := range t.Messages {
			originalChars += len(m.Content)
		}
	}
	return CompactionArchive{
		Summary:    summary,
		WindowTail: keptFromIndex,
		CreatedAt:  time.Now(),
		TurnCount:  keptFromIndex,
		CharSaved:  originalChars - len(summary),
	}
}


// mergeSummaryWithExisting 将新摘要与已有摘要合并
func mergeSummaryWithExisting(existingSummary, newSummary string) string {
	if existingSummary == "" {
		return newSummary
	}
	return existingSummary + "\n\n[Earlier context summary]:\n" + newSummary
}

// modelTokenDefaults lists known context window sizes (in tokens) for popular models.
// Keys are lowercase model ID prefixes for case-insensitive matching.
var modelTokenDefaults = map[string]int{
	// OpenAI
	"gpt-4o":            128000,
	"gpt-4-turbo":       128000,
	"gpt-4-0":           8192,
	"gpt-4-32k":         32768,
	"gpt-3.5-turbo":     16385,
	"o1":                200000,
	"o3":                200000,
	"o4-mini":           200000,
	// Anthropic
	"claude-sonnet-4":   200000,
	"claude-opus-4":     200000,
	"claude-haiku-4":    200000,
	"claude-3.5-sonnet": 200000,
	"claude-3.5-haiku":  200000,
	"claude-3-opus":     200000,
	"claude-3-haiku":    200000,
	// Google
	"gemini-2.5-pro":    1048576,
	"gemini-2.5-flash":  1048576,
	"gemini-2.0-flash":  1048576,
	"gemini-1.5-pro":    2097152,
	"gemini-1.5-flash":  1048576,
	// DeepSeek
	"deepseek-chat":     131072,
	"deepseek-reasoner": 131072,
	// Mistral
	"mistral-large":     131072,
	"mistral-medium":    32768,
	"mistral-small":     32768,
	"codestral":         32768,
}

// getModelContextTokenLimit returns the context window token limit for the given adapter.
// Priority: 1) explicit ContextTokenLimit config, 2) known model defaults, 3) fallback.
func getModelContextTokenLimit(opts AdapterOpts, model string) int {
	if opts.ContextTokenLimit > 0 {
		return opts.ContextTokenLimit
	}
	if limit := lookupModelTokenLimit(model); limit > 0 {
		return limit
	}
	return defaultContextTokenLimit
}

// lookupModelTokenLimit searches the known model defaults table for a matching model ID.
func lookupModelTokenLimit(modelID string) int {
	if modelID == "" {
		return 0
	}
	lower := strings.ToLower(modelID)
	for prefix, limit := range modelTokenDefaults {
		if strings.HasPrefix(lower, prefix) {
			return limit
		}
	}
	return 0
}

// isContextOverflowError 检测provider返回的错误是否为上下文溢出
func isContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "maximum context length") ||
		strings.Contains(msg, "too many tokens") ||
		strings.Contains(msg, "context window") ||
		strings.Contains(msg, "token limit") ||
		strings.Contains(msg, "input is too long") ||
		strings.Contains(msg, "reduce the length") ||
		strings.Contains(msg, "exceeds the maximum") ||
		strings.Contains(msg, "request too large")
}
