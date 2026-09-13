package agent

import (
	"encoding/json"
	"log/slog"

	"charm.land/fantasy"
)

// requestBreakdown captures the estimated composition of a single LLM
// request. This is temporary instrumentation for diagnosing token usage.
type requestBreakdown struct {
	SessionID          string
	StepNumber         int
	SystemPromptTokens int64
	HistoryTokens      int64
	ToolSchemaTokens   int64
	ToolCount          int
	MessageCount       int
}

func (b requestBreakdown) TotalTokens() int64 {
	return b.SystemPromptTokens + b.HistoryTokens + b.ToolSchemaTokens
}

// requestTokenBreakdown estimates how many tokens each request component
// contributes using the existing approxTokenCount heuristic. The values are
// approximate; the provider-reported usage logged by logStepUsage is
// authoritative.
func requestTokenBreakdown(sessionID string, stepNumber int, messages []fantasy.Message, tools []fantasy.AgentTool) requestBreakdown {
	b := requestBreakdown{
		SessionID:    sessionID,
		StepNumber:   stepNumber,
		ToolCount:    len(tools),
		MessageCount: len(messages),
	}
	for _, msg := range messages {
		tokens := estimateMessageTokens([]fantasy.Message{msg})
		if msg.Role == fantasy.MessageRoleSystem {
			b.SystemPromptTokens += tokens
			continue
		}
		b.HistoryTokens += tokens
	}
	for _, t := range tools {
		info := t.Info()
		raw, err := json.Marshal(info)
		if err != nil {
			b.ToolSchemaTokens += approxTokenCount(info.Name) + approxTokenCount(info.Description)
			continue
		}
		b.ToolSchemaTokens += approxTokenCount(string(raw))
	}
	return b
}

func logRequestTokenBreakdown(b requestBreakdown) {
	slog.Debug(
		"LLM request token breakdown (estimated)",
		"session_id", b.SessionID,
		"step", b.StepNumber,
		"system_prompt_tokens", b.SystemPromptTokens,
		"history_tokens", b.HistoryTokens,
		"tool_schema_tokens", b.ToolSchemaTokens,
		"tool_count", b.ToolCount,
		"message_count", b.MessageCount,
		"total_tokens", b.TotalTokens(),
	)
}

// logStepUsage records the provider-reported token usage for a completed step
// alongside the estimated request composition, so real before/after
// comparisons can be made without relying on the estimator. When estimated is
// true the provider returned zero usage and the values come from the fallback
// estimator.
func logStepUsage(b requestBreakdown, usage fantasy.Usage, estimated bool) {
	slog.Debug(
		"LLM step real token usage",
		"session_id", b.SessionID,
		"step", b.StepNumber,
		"estimated", estimated,
		"input_tokens", usage.InputTokens,
		"output_tokens", usage.OutputTokens,
		"reasoning_tokens", usage.ReasoningTokens,
		"cache_creation_tokens", usage.CacheCreationTokens,
		"cache_read_tokens", usage.CacheReadTokens,
		"provider_total_tokens", usage.TotalTokens,
		"prompt_tokens_total", usage.InputTokens+usage.CacheCreationTokens+usage.CacheReadTokens,
		"est_system_prompt_tokens", b.SystemPromptTokens,
		"est_history_tokens", b.HistoryTokens,
		"est_tool_schema_tokens", b.ToolSchemaTokens,
		"tool_count", b.ToolCount,
	)
}
