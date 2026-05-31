package gitclaw

import (
	"strings"
	"testing"
)

func TestIdempotencyKeyUsesStableTriggerIdentity(t *testing.T) {
	ev := Event{
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 12,
			Title:  "@gitclaw hi",
		},
		Comment: &Comment{ID: 345},
		SHA:     "abc123",
	}
	key1 := IdempotencyKey(ev)
	key2 := IdempotencyKey(ev)
	if key1 == "" {
		t.Fatalf("idempotency key is empty")
	}
	if key1 != key2 {
		t.Fatalf("idempotency key not stable: %q != %q", key1, key2)
	}

	ev.SHA = "def456"
	if key1 == IdempotencyKey(ev) {
		t.Fatalf("changing the repo SHA should change the idempotency key")
	}
}

func TestIdempotencyKeyUsesWorkflowDispatchID(t *testing.T) {
	ev := Event{
		Kind:       EventWorkflowDispatch,
		EventName:  "workflow_dispatch",
		Repo:       "owner/repo",
		Issue:      Issue{Number: 12},
		DispatchID: "slack-event-123",
		SHA:        "abc123",
	}
	key1 := IdempotencyKey(ev)
	ev.SHA = "def456"
	if key1 != IdempotencyKey(ev) {
		t.Fatalf("workflow_dispatch ID should remain idempotent across repo SHA changes")
	}
	ev.SHA = "abc123"
	ev.DispatchID = "slack-event-456"
	key2 := IdempotencyKey(ev)
	if key1 == "" || key2 == "" {
		t.Fatalf("dispatch idempotency keys should not be empty")
	}
	if key1 == key2 {
		t.Fatalf("different dispatch IDs should produce different keys")
	}
}

func TestRenderAssistantCommentIncludesMarker(t *testing.T) {
	marker := Marker{
		RunID:               "123",
		EventID:             "456",
		Model:               "fake",
		IdempotencyKey:      "abc",
		RunURL:              "https://github.com/owner/repo/actions/runs/123",
		PromptContextSHA:    "abc123def456",
		ContextDocuments:    2,
		SelectedSkills:      1,
		ToolOutputs:         1,
		PromptVisibleSkills: []string{"repo-reader"},
		PromptVisibleTools:  []string{"gitclaw.search_files"},
		Usage: LLMUsage{
			Present:          true,
			PromptTokens:     120,
			CompletionTokens: 30,
			TotalTokens:      150,
			CacheReadTokens:  80,
		},
	}
	body := RenderAssistantComment(marker, "Hello.")
	if !HasGitClawMarker(body) {
		t.Fatalf("rendered comment does not contain GitClaw marker: %s", body)
	}
	if !ContainsIdempotencyKey(body, "abc") {
		t.Fatalf("rendered comment does not contain idempotency key: %s", body)
	}
	for _, want := range []string{`prompt_context_sha256_12="abc123def456"`, `context_documents="2"`, `selected_skills="1"`, `tool_outputs="1"`, `skills="repo-reader"`, `tools="gitclaw.search_files"`, `usage_prompt_tokens="120"`, `usage_completion_tokens="30"`, `usage_total_tokens="150"`, `usage_cache_read_tokens="80"`, `usage_cache_write_tokens="0"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered comment missing %q: %s", want, body)
		}
	}
}

func TestRenderHeartbeatCommentIncludesModelTelemetry(t *testing.T) {
	body := RenderHeartbeatComment(HeartbeatMarker{
		RunID:               "123",
		Slot:                "slot-1",
		RunURL:              "https://github.com/owner/repo/actions/runs/123",
		Model:               "openai/gpt-5-nano",
		PromptContextSHA:    "abc123def456",
		ContextDocuments:    2,
		SelectedSkills:      1,
		ToolOutputs:         1,
		PromptVisibleSkills: []string{"repo-reader"},
		PromptVisibleTools:  []string{"gitclaw.search_files"},
		Usage: LLMUsage{
			Present:          true,
			PromptTokens:     120,
			CompletionTokens: 30,
			TotalTokens:      150,
			CacheReadTokens:  80,
		},
	}, "Heartbeat.")
	if !HasHeartbeatMarker(body) || !ContainsHeartbeatSlot(body, "slot-1") {
		t.Fatalf("rendered heartbeat comment missing marker: %s", body)
	}
	for _, want := range []string{`model="openai/gpt-5-nano"`, `prompt_context_sha256_12="abc123def456"`, `context_documents="2"`, `selected_skills="1"`, `tool_outputs="1"`, `skills="repo-reader"`, `tools="gitclaw.search_files"`, `usage_prompt_tokens="120"`, `usage_completion_tokens="30"`, `usage_total_tokens="150"`, `usage_cache_read_tokens="80"`, `usage_cache_write_tokens="0"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered heartbeat comment missing %q: %s", want, body)
		}
	}
}

func TestRenderErrorCommentIncludesMarkerWithoutIdempotencyKey(t *testing.T) {
	body := RenderErrorComment(ErrorMarker{
		RunID:   "123",
		EventID: "issue-1",
		Phase:   "model",
		RunURL:  "https://github.com/owner/repo/actions/runs/123",
	}, "model provider request failed")
	if !HasGitClawErrorMarker(body) {
		t.Fatalf("rendered error comment missing marker: %s", body)
	}
	if ContainsIdempotencyKey(body, "abc") || HasGitClawMarker(body) {
		t.Fatalf("error comment should not look like a completed assistant turn: %s", body)
	}
}

func TestMarkerAttributeRequiresExactAttributeName(t *testing.T) {
	attrs := `selected_skills="1" skills="repo-reader" tool_outputs="2" tools="gitclaw.search_files"`
	if got := markerAttribute(attrs, "skills"); got != "repo-reader" {
		t.Fatalf("markerAttribute(skills) = %q, want repo-reader", got)
	}
	if got := markerAttribute(attrs, "tool_outputs"); got != "2" {
		t.Fatalf("markerAttribute(tool_outputs) = %q, want 2", got)
	}
}
