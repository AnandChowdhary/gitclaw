package gitclaw

import "testing"

func TestParseIssueOpenedTrustedTrigger(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 42,
			"title": "@gitclaw explain auth",
			"body": "How does auth work?",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": []
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	if ev.Kind != EventIssueOpened {
		t.Fatalf("kind = %v, want %v", ev.Kind, EventIssueOpened)
	}
	decision := Preflight(ev, DefaultConfig())
	if !decision.Allowed {
		t.Fatalf("trusted prefixed issue should be allowed: %+v", decision)
	}
}

func TestPreflightRejectsPRComment(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 9,
			"title": "@gitclaw review",
			"body": "",
			"author_association": "MEMBER",
			"pull_request": {"url": "https://api.github.com/repos/owner/repo/pulls/9"},
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 1001,
			"body": "@gitclaw follow up",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	decision := Preflight(ev, DefaultConfig())
	if decision.Allowed {
		t.Fatalf("PR comment should be rejected")
	}
	if decision.Code != "pr_comment_ignored" {
		t.Fatalf("code = %q, want pr_comment_ignored", decision.Code)
	}
}

func TestPreflightRejectsUntrustedCommentBeforeLLM(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 7,
			"title": "@gitclaw explain auth",
			"body": "",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 2002,
			"body": "please run",
			"author_association": "CONTRIBUTOR",
			"user": {"login": "mallory", "type": "User"}
		},
		"sender": {"login": "mallory", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	decision := Preflight(ev, DefaultConfig())
	if decision.Allowed {
		t.Fatalf("untrusted commenter should be rejected")
	}
	if decision.Code != "actor_not_trusted" {
		t.Fatalf("code = %q, want actor_not_trusted", decision.Code)
	}
}

func TestPreflightRejectsBotLoop(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 7,
			"title": "@gitclaw explain auth",
			"body": "",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 3003,
			"body": "<!-- gitclaw:assistant-turn idempotency_key=abc -->done",
			"author_association": "MEMBER",
			"user": {"login": "github-actions[bot]", "type": "Bot"}
		},
		"sender": {"login": "github-actions[bot]", "type": "Bot"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	decision := Preflight(ev, DefaultConfig())
	if decision.Allowed {
		t.Fatalf("bot comment should be rejected")
	}
	if decision.Code != "bot_comment_ignored" {
		t.Fatalf("code = %q, want bot_comment_ignored", decision.Code)
	}
}

func TestParseWorkflowDispatchEvent(t *testing.T) {
	ev, err := ParseEvent("workflow_dispatch", []byte(`{
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"inputs": {
			"issue_number": "42",
			"dispatch_id": "telegram-update-123",
			"reason": "channel-poller"
		},
		"sender": {"login": "github-actions[bot]", "type": "Bot"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	if ev.Kind != EventWorkflowDispatch {
		t.Fatalf("kind = %v, want %v", ev.Kind, EventWorkflowDispatch)
	}
	if ev.Issue.Number != 42 || ev.DispatchID != "telegram-update-123" || ev.DispatchReason != "channel-poller" {
		t.Fatalf("unexpected workflow_dispatch event: %#v", ev)
	}
}

func TestPreflightAllowsResolvedWorkflowDispatchFromActionsBot(t *testing.T) {
	ev := Event{
		Kind:      EventWorkflowDispatch,
		EventName: "workflow_dispatch",
		Repo:      "owner/repo",
		Issue: Issue{
			Number:            42,
			Title:             "Mirrored channel message",
			Body:              "Please answer when dispatched.",
			AuthorAssociation: "CONTRIBUTOR",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			Labels:            []string{"gitclaw"},
		},
		Sender: User{Login: "github-actions[bot]", Type: "Bot"},
	}
	decision := Preflight(ev, DefaultConfig())
	if !decision.Allowed {
		t.Fatalf("workflow_dispatch should trust the Actions dispatch boundary: %+v", decision)
	}
}

func TestPreflightAllowsChannelThreadDispatchWithoutTriggerLabel(t *testing.T) {
	ev := Event{
		Kind:      EventWorkflowDispatch,
		EventName: "workflow_dispatch",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 12,
			Title:  "GitClaw telegram thread chat-123",
			Body: RenderChannelThreadBody(ChannelIngestOptions{
				Channel:  "telegram",
				ThreadID: "chat-123",
			}),
			AuthorAssociation: "NONE",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			Labels:            []string{"gitclaw:channel"},
		},
		Sender: User{Login: "github-actions[bot]", Type: "Bot"},
	}
	decision := Preflight(ev, DefaultConfig())
	if !decision.Allowed {
		t.Fatalf("channel thread dispatch should be allowed without trigger label: %+v", decision)
	}
}
