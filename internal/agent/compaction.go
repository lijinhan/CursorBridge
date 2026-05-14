package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// 默认上下文窗口大小（字符数估算），当未从模型配置获取时使用
	defaultContextCharBudget = 180000

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
func getCompactionState(deps *AgentDeps, conversationID string) *CompactionState {
	deps.CompactionStatesMu.Lock()
	defer deps.CompactionStatesMu.Unlock()
	if cs, ok := deps.CompactionStates[conversationID]; ok {
		return cs
	}
	cs := &CompactionState{}
	deps.CompactionStates[conversationID] = cs
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

// getModelContextTokenLimit 从adapter选项或默认值获取模型上下文窗口大小
// TODO: 当AdapterOpts添加ContextTokenLimit字段后，优先使用配置值
func getModelContextTokenLimit(opts AdapterOpts) int {
	return defaultContextCharBudget * 2 / 5 // 默认约72000 tokens
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
