package agent

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLogRequestTokenBreakdown(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	msgs := []fantasy.Message{
		fantasy.NewSystemMessage(strings.Repeat("system prompt text ", 100)),
		fantasy.NewUserMessage(strings.Repeat("user message text ", 100)),
	}
	tools := []fantasy.AgentTool{
		&fakeTool{name: "bash", description: strings.Repeat("desc ", 50)},
		&fakeTool{name: "edit", description: "short"},
	}

	logRequestTokenBreakdown(requestTokenBreakdown("test-session", 0, msgs, tools))

	out := buf.String()
	t.Log(out)
	if !strings.Contains(out, "LLM request token breakdown") {
		t.Fatalf("expected breakdown log, got: %s", out)
	}
	if !strings.Contains(out, "system_prompt_tokens=") || !strings.Contains(out, "tool_schema_tokens=") {
		t.Fatalf("missing fields: %s", out)
	}
}

// TestBaselineTokenOverhead builds the real coder agent (system prompt plus the
// full default tool set) and logs the static per-request token estimate. It
// makes no network calls; it exists to measure how much context crush spends
// before any conversation history.
func TestBaselineTokenOverhead(t *testing.T) {
	env := testEnv(t)

	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 200000, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
	}

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]

	sa, err := coord.buildAgent(t.Context(), p, agentCfg, false)
	require.NoError(t, err)
	require.NoError(t, coord.readyWg.Wait())

	concrete, ok := sa.(*sessionAgent)
	require.True(t, ok, "unexpected SessionAgent type %T", sa)

	systemPrompt := concrete.systemPrompt.Get()
	toolList := concrete.tools.Copy()

	names := make([]string, 0, len(toolList))
	for _, tl := range toolList {
		names = append(names, tl.Info().Name)
	}
	t.Logf("tools (%d): %s", len(names), strings.Join(names, ", "))

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	msgs := []fantasy.Message{
		fantasy.NewSystemMessage(systemPrompt),
		fantasy.NewUserMessage("hello"),
	}
	b := requestTokenBreakdown("baseline", 0, msgs, toolList)
	logRequestTokenBreakdown(b)
	logStepUsage(b, fantasy.Usage{
		InputTokens:         1234,
		OutputTokens:        56,
		CacheCreationTokens: 789,
		CacheReadTokens:     321,
		TotalTokens:         2400,
	}, false)

	t.Logf("system prompt bytes: %d, tools: %d", len(systemPrompt), len(toolList))
	t.Log(buf.String())
}
