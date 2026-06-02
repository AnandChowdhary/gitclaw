package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelMemoryRehearsalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory: durable channel facts.\nCHANNEL_MEMORY_REHEARSAL_TARGET_SECRET\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-06-01.md", "Daily note for channel memory rehearsal.\n")
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "slack",
		ThreadID: "channel-memory-rehearsal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 489,
			"title": "GitClaw slack thread channel-memory-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"slack\" thread_id=\"channel-memory-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48901,
			"body": "@gitclaw /channels rehearse-memory --target long-term --id channel-memory-review --message-id inbound-489 --notify-message-id notify-489\nPlease rehearse this channel-origin memory request.\nCHANNEL_MEMORY_REHEARSAL_SOURCE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 489,
			Title:  "GitClaw slack thread channel-memory-rehearsal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{489: {{
			ID: 48900,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "slack",
				ThreadID:  "channel-memory-rehearsal-thread-123",
				MessageID: "inbound-489",
				Author:    "slack",
				Body:      "Original mirrored message with CHANNEL_MEMORY_REHEARSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{489: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel memory rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one memory rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsalIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:memory-rehearsal-issue",
		`id="channel-memory-review"`,
		`target_kind="long-term"`,
		"rehearsal_id: channel-memory-review",
		"target_kind: long-term",
		"target_path: .gitclaw/MEMORY.md",
		"source_issue: #489",
		"source_kind: channel_comment",
		"memory_validation_status: ok",
		"rehearsal_mode: github-issue-conversation",
		"memory_write_allowed: false",
		"candidate_memory_generation_allowed: false",
		"repository_mutation_allowed: false",
		"raw_source_body_included: false",
		"raw_target_memory_included: false",
		"raw_candidate_memory_included: false",
	} {
		if !strings.Contains(rehearsalIssue.Body, want) {
			t.Fatalf("memory rehearsal issue missing %q:\n%s", want, rehearsalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_REHEARSAL_SOURCE_SECRET", "CHANNEL_MEMORY_REHEARSAL_INGEST_SECRET", "CHANNEL_MEMORY_REHEARSAL_TARGET_SECRET", "Please rehearse this channel-origin"} {
		if strings.Contains(rehearsalIssue.Body, leaked) {
			t.Fatalf("memory rehearsal issue leaked %q:\n%s", leaked, rehearsalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[489]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="slack"`,
		`message_id="notify-489"`,
		"GitClaw channel memory rehearsal",
		"Rehearsal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Target: .gitclaw/MEMORY.md",
		"Validation: ok",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel memory rehearsal notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_MEMORY_REHEARSAL_SOURCE_SECRET") || strings.Contains(outbound, "CHANNEL_MEMORY_REHEARSAL_INGEST_SECRET") || strings.Contains(outbound, "CHANNEL_MEMORY_REHEARSAL_TARGET_SECRET") {
		t.Fatalf("channel memory rehearsal notification leaked source:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Memory Rehearsal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels rehearse-memory`",
		"channel_memory_rehearsal_status: `created`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#489`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"normalized_target_kind: `long-term`",
		"normalized_target_path: `.gitclaw/MEMORY.md`",
		"memory_validation_status: `ok`",
		"rehearsal_mode: `github-issue-conversation`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"memory_write_allowed: `false`",
		"candidate_memory_generation_allowed: `false`",
		"memory_file_written: `false`",
		"raw_rehearsal_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_target_memory_included: `false`",
		"raw_candidate_memory_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_memory_rehearsal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel memory rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_REHEARSAL_SOURCE_SECRET", "CHANNEL_MEMORY_REHEARSAL_INGEST_SECRET", "CHANNEL_MEMORY_REHEARSAL_TARGET_SECRET", "Please rehearse this channel-origin", "channel-memory-review", "channel-memory-rehearsal-thread-123", "inbound-489", "notify-489", "Memory: durable channel facts"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel memory rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 489,
			"title": "GitClaw slack thread channel-memory-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"slack\" thread_id=\"channel-memory-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48902,
			"body": "@gitclaw /channels rehearse-memory --target long-term --id channel-memory-review --message-id inbound-489 --notify-message-id notify-489\nDo not leak duplicate token CHANNEL_MEMORY_REHEARSAL_DUPLICATE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	if err := Handle(context.Background(), duplicateEv, cfg, github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("duplicate channel memory rehearsal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[489]); got != 4 {
		t.Fatalf("duplicate channel memory rehearsal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[489])
	}
	duplicateReceipt := github.CommentsByIssue[489][3].Body
	for _, want := range []string{
		"channel_memory_rehearsal_status: `duplicate`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel memory rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_MEMORY_REHEARSAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate channel memory rehearsal receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelMemoryRehearsalActionRequestParsesAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory: durable facts for tests.\n")
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContext(root, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 33, Title: "Channel memory rehearsal"},
		Comment: &Comment{
			ID:   3301,
			Body: `@gitclaw /channel recall-test --target Memory --id Channel.Memory.Rehearsal --channel telegram --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelMemoryRehearsalActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelMemoryRehearsalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "recall-test" || req.Options.Channel != "telegram" || req.Options.RehearsalID != "channel-memory-rehearsal" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel memory rehearsal parsing: %#v", req)
	}
	if req.Rehearsal.Target.Kind != "long-term" || req.Rehearsal.Target.Path != ".gitclaw/MEMORY.md" || req.Rehearsal.SourceKind != "channel_comment" || req.TargetFromIssue || req.AutoRehearsalID || req.AutoNotifyMessageID {
		t.Fatalf("unexpected memory rehearsal details: %#v", req)
	}
}
