package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelProfileStatusQueuesSnapshotWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelProfileStatusFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-profile-status-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 889,
			"title": "GitClaw telegram thread chat-profile-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-profile-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88901,
			"body": "@gitclaw /channels profile-status --message-id profile-status-inbound-889 --notify-message-id profile-status-notify-889 --status-id profile-status-secret-889\nDo not include this command hidden token in the receipt: CHANNEL_PROFILE_STATUS_COMMAND_SECRET.",
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
			Number: 889,
			Title:  "GitClaw telegram thread chat-profile-status-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{889: {{
			ID: 88900,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-profile-status-123",
				MessageID: "profile-status-inbound-889",
				Author:    "telegram",
				Body:      "Original mirrored profile command with CHANNEL_PROFILE_STATUS_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{889: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel profile status action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("profile status should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[889]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="profile-status-notify-889"`,
		"GitClaw channel profile status.",
		"Profile status: ok",
		"Profile store: .gitclaw/",
		"Profile scope: repository",
		"Snapshot version: gitclaw-profile-snapshot-v1",
		"Snapshot hash: ",
		"Snapshot entries: 5",
		"Profile documents loaded: 6",
		"Required profile documents: 6/6 present",
		"Available skills: 1",
		"Selected skills: 0",
		"Skill bundles: 0",
		"Available tools: 5",
		"Active tool outputs: 2",
		"Components: manifest=ok soul=ok memory=ok skills=ok tools=ok",
		"Profile export: disabled.",
		"Profile import: disabled.",
		"Profile switching: disabled.",
		"Profile mutation: disabled.",
		"External profile home: not accessed.",
		"Credentials: not included.",
		"Sessions: not included.",
		"Backup payloads: not included.",
		"Raw profile bodies: not included.",
		"Raw skill bodies: not included.",
		"Raw memory bodies: not included.",
		"Raw tool outputs: not included.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("profile-status notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_PROFILE_STATUS_INGEST_SECRET", "CHANNEL_PROFILE_STATUS_COMMAND_SECRET", "CHANNEL_PROFILE_STATUS_SOUL_SECRET", "CHANNEL_PROFILE_STATUS_SKILL_SECRET", "profile-status-secret-889"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("profile-status notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Profile Status Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels profile-status`",
		"channel_profile_status_status: `queued`",
		"profile_snapshot_mode: `provider-facing-profile-status`",
		"notification_target_issue: `#889`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"profile_status: `ok`",
		"profile_strategy: `repo-local-git-profile`",
		"profile_store: `.gitclaw/`",
		"profile_scope: `repository`",
		"snapshot_version: `gitclaw-profile-snapshot-v1`",
		"snapshot_scope: `repo-local-profile-soul-memory-skills-tools`",
		"snapshot_sha256_12: `",
		"snapshot_entries: `5`",
		"profile_documents_loaded: `6`",
		"required_profile_documents: `6`",
		"required_profile_documents_present: `6`",
		"required_profile_documents_missing: `0`",
		"available_skills: `1`",
		"selected_skills: `0`",
		"skill_bundles: `0`",
		"available_tools: `5`",
		"active_tool_outputs: `2`",
		"manifest_entries: `13`",
		"profile_manifest_sha256_12: `",
		"soul_snapshot_sha256_12: `",
		"memory_snapshot_sha256_12: `",
		"skill_snapshot_sha256_12: `",
		"tool_snapshot_sha256_12: `",
		"soul_status: `ok`",
		"memory_status: `ok`",
		"skill_status: `ok`",
		"tool_status: `ok`",
		"profile_export_supported: `false`",
		"profile_import_supported: `false`",
		"profile_switching_supported: `false`",
		"profile_mutation_allowed: `false`",
		"credentials_included: `false`",
		"sessions_included: `false`",
		"backup_payloads_included: `false`",
		"notification_body_sha256_12: `",
		"profile_export_performed: `false`",
		"profile_import_performed: `false`",
		"profile_switch_performed: `false`",
		"profile_mutation_performed: `false`",
		"external_profile_home_accessed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_profile_status_id_included: `false`",
		"raw_profile_file_paths_included: `false`",
		"raw_profile_bodies_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_memory_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_session_bodies_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_profile_status_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel profile status receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PROFILE_STATUS_INGEST_SECRET", "CHANNEL_PROFILE_STATUS_COMMAND_SECRET", "CHANNEL_PROFILE_STATUS_SOUL_SECRET", "CHANNEL_PROFILE_STATUS_SKILL_SECRET", "chat-profile-status-123", "profile-status-inbound-889", "profile-status-notify-889", "profile-status-secret-889", ".gitclaw/SOUL.md", ".gitclaw/SKILLS/repo-reader/SKILL.md"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel profile status receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 889,
			"title": "GitClaw telegram thread chat-profile-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-profile-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88902,
			"body": "@gitclaw /channels agent-profile --message-id profile-status-inbound-889 --notify-message-id profile-status-notify-889 --status-id profile-status-secret-889\nDo not leak duplicate token CHANNEL_PROFILE_STATUS_DUPLICATE_SECRET.",
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
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate profile status created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[889]); got != 4 {
		t.Fatalf("duplicate profile status posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[889])
	}
	duplicateReceipt := github.CommentsByIssue[889][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels agent-profile`",
		"channel_profile_status_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"profile_export_performed: `false`",
		"profile_import_performed: `false`",
		"profile_switch_performed: `false`",
		"profile_mutation_performed: `false`",
		"external_profile_home_accessed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate profile status receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PROFILE_STATUS_DUPLICATE_SECRET", "chat-profile-status-123", "profile-status-inbound-889", "profile-status-notify-889", "profile-status-secret-889"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate profile status receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelProfileStatusActionRequestParsesRouteAlias(t *testing.T) {
	root := t.TempDir()
	writeChannelProfileStatusFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channels profile-status"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel profile status"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel context-profile --route team-demo --message-id source-1 --notify-message-id notify-1 --profile-status-id Profile.Status`,
		},
	}
	req, err := BuildChannelProfileStatusActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelProfileStatusActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "context-profile" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StatusID != "profile-status" {
		t.Fatalf("unexpected channel profile status parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStatusID || req.RequestedRouteHash == "" || req.StatusIDHash == "" || req.ProfileStatus != "ok" || req.SnapshotEntries != 5 || req.ProfileDocumentsLoaded != 6 || req.AvailableSkills != 1 || req.AvailableTools != 5 || req.SnapshotSHA == "" || req.ProfileManifestSHA == "" || req.SoulSnapshotSHA == "" || req.MemorySnapshotSHA == "" || req.SkillSnapshotSHA == "" || req.ToolSnapshotSHA == "" {
		t.Fatalf("expected explicit route profile-status hashes and counts: %#v", req)
	}
}

func writeChannelProfileStatusFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Soul fixture. Hidden token CHANNEL_PROFILE_STATUS_SOUL_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Identity fixture.\n")
	writeTestFile(t, root, ".gitclaw/USER.md", "User fixture.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Tools fixture.\n")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory fixture.\n")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "Heartbeat fixture.\n")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Read repository context.
---

# Repo Reader

Hidden token CHANNEL_PROFILE_STATUS_SKILL_SECRET.
`)
	writeTestFile(t, root, "README.md", "fixture repo\n")
}
