package gitclaw

import "testing"

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
		RunID:          "123",
		EventID:        "456",
		Model:          "fake",
		IdempotencyKey: "abc",
		RunURL:         "https://github.com/owner/repo/actions/runs/123",
	}
	body := RenderAssistantComment(marker, "Hello.")
	if !HasGitClawMarker(body) {
		t.Fatalf("rendered comment does not contain GitClaw marker: %s", body)
	}
	if !ContainsIdempotencyKey(body, "abc") {
		t.Fatalf("rendered comment does not contain idempotency key: %s", body)
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
