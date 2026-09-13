package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// coderPromptBudget is the maximum size of the coder system prompt template in
// bytes. The prompt is part of every request and is cached as a prefix, so
// growth here directly increases per-request token usage. Raise this budget
// only deliberately, and prefer trimming redundant instructions first.
const coderPromptBudget = 16000

// TestCoderPromptSizeBudget guards against prompt growth. The coder template is
// sent on every request; when it was last reviewed it measured ~12 KB. If this
// test fails, either trim the prompt back down or bump the budget in the same
// change with a note explaining why the extra tokens are worth it.
func TestCoderPromptSizeBudget(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, coderPromptTmpl, "coder prompt template is empty")
	require.LessOrEqual(t, len(coderPromptTmpl), coderPromptBudget,
		"coder.md.tpl grew past the %d-byte budget (currently %d bytes); "+
			"trim it or raise coderPromptBudget deliberately",
		coderPromptBudget, len(coderPromptTmpl))
}
