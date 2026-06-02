package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSoulInfoQueuesFocusedCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelSoulInfoFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-soul-info-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 905,
			"title": "GitClaw telegram thread chat-soul-info-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-info-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90501,
			"body": "@gitclaw /channels soul-info SOUL --message-id soul-info-inbound-905 --notify-message-id soul-info-notify-905 --info-id Soul.Info.Secret.905\nDo not include this command hidden token in the receipt: CHANNEL_SOUL_INFO_COMMAND_MARKER.",
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
			Number: 905,
			Title:  "GitClaw telegram thread chat-soul-info-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{905: {{
			ID: 90500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-soul-info-123",
				MessageID: "soul-info-inbound-905",
				Author:    "telegram",
				Body:      "Original mirrored soul info command with CHANNEL_SOUL_INFO_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{905: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel soul info action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("soul info action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[905]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="soul-info-notify-905"`,
		"GitClaw channel soul info",
		"Soul info status: ok",
		"Requested path hash: ",
		"Normalized path hash: ",
		"Matched soul files: 1",
		"Context documents: 6",
		"Validation status: ok",
		"Validation errors: 0",
		"Validation warnings: 0",
		"Required files: 6",
		"Present required files: 6",
		"Missing required files: 0",
		"Risk status: ok",
		"Risk findings: 0",
		"Soul info id hash: ",
		"path=.gitclaw/SOUL.md",
		"category=soul",
		"source=repo-local",
		"present=true",
		"required=true",
		"canonical=true",
		"loaded_for_this_turn=true",
		"sha256_12=",
		"Raw soul, identity, user, memory, tool guidance, heartbeat, channel, issue, comment, prompt, and tool output bodies are not included.",
		"Model call: not performed by this action.",
		"Soul write: not performed by this action.",
		"Memory write: not performed by this action.",
		"Registry contact: not performed by this action.",
		"Profile export: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("soul info notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_INFO_SOUL_SECRET", "Soul.Info.Secret.905"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("soul info notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Soul Info Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels soul-info`",
		"channel_soul_info_status: `queued`",
		"soul_info_status: `ok`",
		"info_mode: `repo-local-high-authority-metadata-card`",
		"notification_target_issue: `#905`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"soul_info_id_sha256_12: `",
		"soul_info_id_auto: `false`",
		"requested_soul_path_sha256_12: `",
		"normalized_soul_path_sha256_12: `",
		"requested_soul_path_bytes: `4`",
		"path_source: `positional`",
		"matched_soul_files: `1`",
		"context_documents: `6`",
		"validation_status: `ok`",
		"validation_errors: `0`",
		"validation_warnings: `0`",
		"required_files: `6`",
		"present_required_files: `6`",
		"missing_required_files: `0`",
		"memory_notes: `0`",
		"risk_status: `ok`",
		"risk_findings: `0`",
		"soul_info_match_sha256_12: `",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"progressive_disclosure_enabled: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"soul_writes_performed: `false`",
		"memory_writes_performed: `false`",
		"profile_export_performed: `false`",
		"registry_contact_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_requested_soul_path_included: `false`",
		"raw_normalized_soul_path_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_soul_info_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_soul_bodies_included: `false`",
		"raw_identity_bodies_included: `false`",
		"raw_user_bodies_included: `false`",
		"raw_memory_bodies_included: `false`",
		"raw_tool_guidance_bodies_included: `false`",
		"raw_heartbeat_bodies_included: `false`",
		"raw_soul_file_paths_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_soul_info_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel soul info receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"SOUL", ".gitclaw/SOUL.md", "CHANNEL_SOUL_INFO_SOUL_SECRET", "CHANNEL_SOUL_INFO_INGEST_MARKER", "CHANNEL_SOUL_INFO_COMMAND_MARKER", "chat-soul-info-123", "soul-info-inbound-905", "soul-info-notify-905", "Soul.Info.Secret.905"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel soul info receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 905,
			"title": "GitClaw telegram thread chat-soul-info-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-info-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90502,
			"body": "@gitclaw /channels authority-info SOUL --message-id soul-info-inbound-905 --notify-message-id soul-info-notify-905 --info-id Soul.Info.Secret.905\nDo not include duplicate hidden token CHANNEL_SOUL_INFO_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[905]); got != 4 {
		t.Fatalf("duplicate soul info posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[905])
	}
	duplicateReceipt := github.CommentsByIssue[905][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels authority-info`",
		"channel_soul_info_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"soul_writes_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate soul info receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"SOUL", ".gitclaw/SOUL.md", "CHANNEL_SOUL_INFO_DUPLICATE_MARKER", "chat-soul-info-123", "soul-info-inbound-905", "soul-info-notify-905", "Soul.Info.Secret.905"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate soul info receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSoulInfoActionRequestParsesRouteAliasAndTrailingPath(t *testing.T) {
	root := t.TempDir()
	writeChannelSoulInfoFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel soul info"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel context-info --route team-demo --message-id source-1 --notify-message-id notify-1 --id Soul.Info.One
Path: identity`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel context-info"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelSoulInfoActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSoulInfoActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "context-info" || req.Options.Route != "team-demo" || req.Options.RequestedPath != "identity" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.InfoID != "soul-info-one" {
		t.Fatalf("unexpected channel soul info parsing: %#v", req)
	}
	if req.PathSource != "trailing-path" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoInfoID {
		t.Fatalf("unexpected channel soul info defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.InfoIDHash == "" || req.RequestedPathHash == "" || req.NormalizedPathHash == "" || req.NotificationBodySHA == "" || req.Info.InfoStatus != "ok" || req.Info.Match.Path != ".gitclaw/IDENTITY.md" {
		t.Fatalf("expected route soul info hashes and match: %#v", req)
	}
	if !IsChannelSoulInfoActionRequest(ev, cfg) {
		t.Fatalf("expected channel context-info alias to be recognized")
	}
}

func writeChannelSoulInfoFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "# Soul\n\n- Repo-native operating boundary. Hidden body token CHANNEL_SOUL_INFO_SOUL_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "# Identity\n\n- Identity context.\n")
	writeTestFile(t, root, ".gitclaw/USER.md", "# User\n\n- User preference.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "# Tools\n\n- Tool guidance.\n")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "# Memory\n\n- Durable memory index.\n")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "# Heartbeat\n\n- Heartbeat notes.\n")
}
