package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHandleChannelSoulRehearsalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		".gitclaw/SOUL.md":              "Voice: direct, warm, pragmatic.\nCHANNEL_SOUL_REHEARSAL_TARGET_SECRET",
		".gitclaw/IDENTITY.md":          "Identity: GitClaw.",
		".gitclaw/USER.md":              "User facts.",
		".gitclaw/TOOLS.md":             "Tools.",
		".gitclaw/MEMORY.md":            "Memory.",
		".gitclaw/HEARTBEAT.md":         "Heartbeat.",
		".gitclaw/memory/2026-06-01.md": "Daily note.",
	}
	for path, body := range files {
		writeTestFile(t, root, path, body)
	}
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-soul-rehearsal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 487,
			"title": "GitClaw telegram thread channel-soul-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-soul-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48701,
			"body": "@gitclaw /channels rehearse-soul --target soul --id channel-soul-review --message-id inbound-487 --notify-message-id notify-487\nPlease rehearse this channel-origin soul request.\nCHANNEL_SOUL_REHEARSAL_SOURCE_SECRET",
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
			Number: 487,
			Title:  "GitClaw telegram thread channel-soul-rehearsal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{487: {{
			ID: 48700,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-soul-rehearsal-thread-123",
				MessageID: "inbound-487",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_SOUL_REHEARSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{487: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel soul rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one soul rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsalIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:soul-rehearsal-issue",
		`id="channel-soul-review"`,
		`target_path=".gitclaw/SOUL.md"`,
		"rehearsal_id: channel-soul-review",
		"requested_target: soul",
		"target_path: .gitclaw/SOUL.md",
		"target_category: soul",
		"source_issue: #487",
		"source_kind: channel_comment",
		"context_target_write_allowed: false",
		"candidate_soul_generation_allowed: false",
		"repository_mutation_allowed: false",
		"raw_source_body_included: false",
		"raw_target_body_included: false",
		"raw_candidate_soul_included: false",
	} {
		if !strings.Contains(rehearsalIssue.Body, want) {
			t.Fatalf("soul rehearsal issue missing %q:\n%s", want, rehearsalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_REHEARSAL_SOURCE_SECRET", "CHANNEL_SOUL_REHEARSAL_INGEST_SECRET", "CHANNEL_SOUL_REHEARSAL_TARGET_SECRET", "Please rehearse this channel-origin", "Voice: direct"} {
		if strings.Contains(rehearsalIssue.Body, leaked) {
			t.Fatalf("soul rehearsal issue leaked %q:\n%s", leaked, rehearsalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[487]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-487"`,
		"GitClaw channel soul rehearsal",
		"Rehearsal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Target: .gitclaw/SOUL.md",
		"Validation: ok",
		"Risk: ok",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel soul rehearsal notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_SOUL_REHEARSAL_SOURCE_SECRET") || strings.Contains(outbound, "CHANNEL_SOUL_REHEARSAL_INGEST_SECRET") || strings.Contains(outbound, "CHANNEL_SOUL_REHEARSAL_TARGET_SECRET") {
		t.Fatalf("channel soul rehearsal notification leaked source:\n%s", outbound)
	}
	outboundParts := strings.SplitN(outbound, "\n", 2)
	if len(outboundParts) != 2 {
		t.Fatalf("channel soul rehearsal outbound missing provider body:\n%s", outbound)
	}
	notificationBody := strings.TrimSpace(outboundParts[1])
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Soul Rehearsal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels rehearse-soul`",
		"channel_soul_rehearsal_status: `created`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#487`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"requested_target: `soul`",
		"normalized_soul_path: `.gitclaw/SOUL.md`",
		"target_category: `soul`",
		"notification_body_sha256_12: `" + shortDocumentHash(notificationBody) + "`",
		fmt.Sprintf("notification_body_bytes: `%d`", len(notificationBody)),
		fmt.Sprintf("notification_body_lines: `%d`", lineCount(notificationBody)),
		"rehearsal_mode: `github-issue-conversation`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"context_target_write_allowed: `false`",
		"candidate_soul_generation_allowed: `false`",
		"soul_file_written: `false`",
		"raw_rehearsal_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_target_body_included: `false`",
		"raw_candidate_soul_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_soul_rehearsal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel soul rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_REHEARSAL_SOURCE_SECRET", "CHANNEL_SOUL_REHEARSAL_INGEST_SECRET", "CHANNEL_SOUL_REHEARSAL_TARGET_SECRET", "Please rehearse this channel-origin", "channel-soul-review", "channel-soul-rehearsal-thread-123", "inbound-487", "notify-487", "Voice: direct"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel soul rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 487,
			"title": "GitClaw telegram thread channel-soul-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-soul-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48702,
			"body": "@gitclaw /channels rehearse-soul --target soul --id channel-soul-review --message-id inbound-487 --notify-message-id notify-487\nDo not leak duplicate token CHANNEL_SOUL_REHEARSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel soul rehearsal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[487]); got != 4 {
		t.Fatalf("duplicate channel soul rehearsal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[487])
	}
	duplicateReceipt := github.CommentsByIssue[487][3].Body
	for _, want := range []string{
		"channel_soul_rehearsal_status: `duplicate`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel soul rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_SOUL_REHEARSAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate channel soul rehearsal receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelSoulRehearsalActionRequestParsesAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Soul policy.")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Identity policy.")
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel tone-test --target Identity --id Channel.Soul.Rehearsal --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 34, Title: "Channel soul rehearsal"},
		Comment: &Comment{
			ID:   3401,
			Body: `@gitclaw /channel tone-test --target Identity --id Channel.Soul.Rehearsal --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelSoulRehearsalActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSoulRehearsalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tone-test" || req.Options.Channel != "slack" || req.Options.RehearsalID != "channel-soul-rehearsal" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel soul rehearsal parsing: %#v", req)
	}
	if req.Rehearsal.TargetPath != ".gitclaw/IDENTITY.md" || req.Rehearsal.RequestedTarget != "Identity" || req.TargetFromIssue || req.AutoRehearsalID || req.AutoNotifyMessageID {
		t.Fatalf("unexpected soul rehearsal details: %#v", req)
	}
}
