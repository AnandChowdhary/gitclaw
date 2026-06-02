package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToolRehearsalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "GitClaw tools are deterministic and reviewed.\nCHANNEL_TOOL_REHEARSAL_TOOL_BODY_SECRET\n")
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-tool-rehearsal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 490,
			"title": "GitClaw telegram thread channel-tool-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-tool-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49001,
			"body": "@gitclaw /channels rehearse-tool search_files --id channel-tool-rehearsal --message-id inbound-490 --notify-message-id notify-490\nPlease rehearse this channel-origin tool boundary.\nCHANNEL_TOOL_REHEARSAL_SOURCE_SECRET",
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
			Number: 490,
			Title:  "GitClaw telegram thread channel-tool-rehearsal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{490: {{
			ID: 49000,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-tool-rehearsal-thread-123",
				MessageID: "inbound-490",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_TOOL_REHEARSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{490: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one tool rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsalIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:tool-rehearsal-issue",
		`id="channel-tool-rehearsal"`,
		`normalized_tool="gitclaw.search_files"`,
		"rehearsal_id: channel-tool-rehearsal",
		"normalized_tool: gitclaw.search_files",
		"matched_tool: gitclaw.search_files",
		"source_issue: #490",
		"source_kind: channel_comment",
		"rehearsal_mode: github-issue-conversation",
		"tool_execution_performed: false",
		"tool_inputs_generated: false",
		"tool_run_request_created: false",
		"repository_mutation_performed: false",
		"raw_source_body_included: false",
		"raw_tool_inputs_included: false",
		"raw_tool_outputs_included: false",
	} {
		if !strings.Contains(rehearsalIssue.Body, want) {
			t.Fatalf("tool rehearsal issue missing %q:\n%s", want, rehearsalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_REHEARSAL_SOURCE_SECRET", "CHANNEL_TOOL_REHEARSAL_INGEST_SECRET", "CHANNEL_TOOL_REHEARSAL_TOOL_BODY_SECRET", "Please rehearse this channel-origin"} {
		if strings.Contains(rehearsalIssue.Body, leaked) {
			t.Fatalf("tool rehearsal issue leaked %q:\n%s", leaked, rehearsalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[490]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-490"`,
		"GitClaw channel tool rehearsal",
		"Rehearsal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Normalized tool: gitclaw.search_files",
		"Matched tool: gitclaw.search_files",
		"Tool enabled: true",
		"Tool mode: read-only",
		"Mutating contract: false",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel tool rehearsal notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_REHEARSAL_SOURCE_SECRET", "CHANNEL_TOOL_REHEARSAL_INGEST_SECRET", "CHANNEL_TOOL_REHEARSAL_TOOL_BODY_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("channel tool rehearsal notification leaked %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Rehearsal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels rehearse-tool`",
		"channel_tool_rehearsal_status: `created`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#490`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"normalized_tool: `gitclaw.search_files`",
		"matched_tool: `gitclaw.search_files`",
		"tool_enabled: `true`",
		"tool_mode: `read-only`",
		"mutating_contract: `false`",
		"rehearsal_mode: `github-issue-conversation`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"tool_inputs_generated: `false`",
		"tool_run_request_created: `false`",
		"raw_rehearsal_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_requested_tool_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_tool_rehearsal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_REHEARSAL_SOURCE_SECRET", "CHANNEL_TOOL_REHEARSAL_INGEST_SECRET", "CHANNEL_TOOL_REHEARSAL_TOOL_BODY_SECRET", "Please rehearse this channel-origin", "channel-tool-rehearsal", "channel-tool-rehearsal-thread-123", "inbound-490", "notify-490", "GitClaw tools are deterministic"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 490,
			"title": "GitClaw telegram thread channel-tool-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-tool-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49002,
			"body": "@gitclaw /channels rehearse-tool search_files --id channel-tool-rehearsal --message-id inbound-490 --notify-message-id notify-490\nDo not leak duplicate token CHANNEL_TOOL_REHEARSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel tool rehearsal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[490]); got != 4 {
		t.Fatalf("duplicate channel tool rehearsal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[490])
	}
	duplicateReceipt := github.CommentsByIssue[490][3].Body
	for _, want := range []string{
		"channel_tool_rehearsal_status: `duplicate`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel tool rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_TOOL_REHEARSAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate channel tool rehearsal receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelToolRehearsalActionRequestParsesAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "GitClaw tools are deterministic and reviewed.\n")
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
		Issue:     Issue{Number: 34, Title: "Channel tool rehearsal"},
		Comment: &Comment{
			ID:   3401,
			Body: `@gitclaw /channel tool-lab --tool Read_File --id Channel.Tool.Rehearsal --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelToolRehearsalActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelToolRehearsalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-lab" || req.Options.Channel != "slack" || req.Options.RehearsalID != "channel-tool-rehearsal" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel tool rehearsal parsing: %#v", req)
	}
	if req.Rehearsal.NormalizedTool != "gitclaw.read_file" || req.Rehearsal.SourceKind != "channel_comment" || req.TargetFromIssue || req.AutoRehearsalID || req.AutoNotifyMessageID {
		t.Fatalf("unexpected tool rehearsal details: %#v", req)
	}
}
