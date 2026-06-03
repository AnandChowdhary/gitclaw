package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSkillSpotlightQueuesCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelSkillSpotlightFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-skill-spotlight-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 908,
			"title": "GitClaw telegram thread chat-skill-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90801,
			"body": "@gitclaw /channels skill-spotlight repo-reader --message-id skill-spotlight-inbound-908 --notify-message-id skill-spotlight-notify-908 --spotlight-id Skill.Spotlight.Secret.908\nDo not include this command hidden token in the receipt: CHANNEL_SKILL_SPOTLIGHT_COMMAND_MARKER.",
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
			Number: 908,
			Title:  "GitClaw telegram thread chat-skill-spotlight-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{908: {{
			ID: 90800,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-skill-spotlight-123",
				MessageID: "skill-spotlight-inbound-908",
				Author:    "telegram",
				Body:      "Original mirrored skill spotlight command with CHANNEL_SKILL_SPOTLIGHT_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{908: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel skill spotlight action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("skill spotlight action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[908]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="skill-spotlight-notify-908"`,
		"GitClaw channel skill spotlight",
		"Spotlight status: ok",
		"Focus hash: ",
		"Focus terms: 1",
		"Available skills: 2",
		"Enabled skills: 2",
		"Eligible skills: 2",
		"Matched skills: 1",
		"Candidate skills: 1",
		"Selected index: 0",
		"Selection seed hash: ",
		"Selection hash: ",
		"Validation status: ok",
		"Skill spotlight id hash: ",
		"Spotlight:",
		"skill_name=repo-reader",
		"path=.gitclaw/SKILLS/repo-reader/SKILL.md",
		"folder=repo-reader",
		"enabled=true",
		"frontmatter=true",
		"description_present=true",
		"sha256_12=",
		"Try next:",
		"@gitclaw /channels skill-info repo-reader",
		"@gitclaw /channels skill-map repo-reader",
		"Raw skill bodies, skill descriptions, channel bodies, issue bodies, comment bodies, prompts, tool outputs, raw focus text, raw notes, and raw spotlight ids are not included in the source receipt.",
		"Skill install: not performed by this action.",
		"Skill update: not performed by this action.",
		"Registry contact: not performed by this action.",
		"Installer scripts: not run by this action.",
		"Tool execution: not performed by this action.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("skill spotlight notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_SPOTLIGHT_INGEST_MARKER", "CHANNEL_SKILL_SPOTLIGHT_COMMAND_MARKER", "CHANNEL_SKILL_SPOTLIGHT_DESCRIPTION_SECRET", "CHANNEL_SKILL_SPOTLIGHT_BODY_SECRET", "Skill.Spotlight.Secret.908", "Use read-only repository context."} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("skill spotlight notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Skill Spotlight Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels skill-spotlight`",
		"channel_skill_spotlight_status: `queued`",
		"skill_spotlight_status: `ok`",
		"spotlight_mode: `repo-local-skill-deterministic-draw`",
		"notification_target_issue: `#908`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"skill_spotlight_id_sha256_12: `",
		"skill_spotlight_id_auto: `false`",
		"spotlight_focus_sha256_12: `",
		"spotlight_focus_bytes: `11`",
		"spotlight_focus_terms: `1`",
		"spotlight_focus_source: `positional`",
		"available_skills: `2`",
		"enabled_skills: `2`",
		"eligible_skills: `2`",
		"matched_skills: `1`",
		"candidate_skills: `1`",
		"selected_index: `0`",
		"selected_skill_name_sha256_12: `",
		"selected_skill_path_sha256_12: `",
		"selected_skill_folder_sha256_12: `",
		"selection_seed_sha256_12: `",
		"selection_sha256_12: `",
		"validation_status: `ok`",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"progressive_disclosure_enabled: `true`",
		"deterministic_selection: `true`",
		"external_randomness_used: `false`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"registry_contact_allowed: `false`",
		"installer_scripts_run: `false`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_focus_included: `false`",
		"raw_note_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_skill_spotlight_id_included: `false`",
		"raw_selection_seed_included: `false`",
		"raw_skill_names_included: `false`",
		"raw_skill_paths_included: `false`",
		"raw_skill_descriptions_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_skill_spotlight_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel skill spotlight receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"repo-reader", ".gitclaw/SKILLS/repo-reader/SKILL.md", "CHANNEL_SKILL_SPOTLIGHT_INGEST_MARKER", "CHANNEL_SKILL_SPOTLIGHT_COMMAND_MARKER", "CHANNEL_SKILL_SPOTLIGHT_DESCRIPTION_SECRET", "CHANNEL_SKILL_SPOTLIGHT_BODY_SECRET", "Use read-only repository context.", "chat-skill-spotlight-123", "skill-spotlight-inbound-908", "skill-spotlight-notify-908", "Skill.Spotlight.Secret.908"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel skill spotlight receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 908,
			"title": "GitClaw telegram thread chat-skill-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90802,
			"body": "@gitclaw /channels skill-draw repo-reader --message-id skill-spotlight-inbound-908 --notify-message-id skill-spotlight-notify-908 --spotlight-id Skill.Spotlight.Secret.908\nDo not leak duplicate hidden token CHANNEL_SKILL_SPOTLIGHT_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[908]); got != 4 {
		t.Fatalf("duplicate skill spotlight posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[908])
	}
	duplicateReceipt := github.CommentsByIssue[908][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels skill-draw`",
		"channel_skill_spotlight_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate skill spotlight receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"repo-reader", "CHANNEL_SKILL_SPOTLIGHT_DUPLICATE_MARKER", "chat-skill-spotlight-123", "skill-spotlight-inbound-908", "skill-spotlight-notify-908", "Skill.Spotlight.Secret.908"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate skill spotlight receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSkillSpotlightActionRequestParsesRouteAliasAndTrailingNote(t *testing.T) {
	root := t.TempDir()
	writeChannelSkillSpotlightFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel skill spotlight"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel capability-draw --route team-demo --message-id source-1 --notify-message-id notify-1 --id Skill.Spotlight.One --focus repo-reader
Note: try a safe reader card.`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel capability-draw"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelSkillSpotlightActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSkillSpotlightActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "capability-draw" || req.Options.Route != "team-demo" || req.Options.Focus != "repo-reader" || req.Options.Note != "try a safe reader card." || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SpotlightID != "skill-spotlight-one" {
		t.Fatalf("unexpected channel skill spotlight parsing: %#v", req)
	}
	if req.FocusSource != "flag" || req.NoteSource != "trailing-note" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSpotlightID {
		t.Fatalf("unexpected channel skill spotlight defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SpotlightIDHash == "" || req.FocusSHA == "" || req.SelectionSHA == "" || req.NotificationBodySHA == "" || req.Report.CandidateSkills != 1 {
		t.Fatalf("expected route spotlight hashes and result: %#v", req)
	}
	if !IsChannelSkillSpotlightActionRequest(ev, cfg) {
		t.Fatalf("expected channel capability-draw alias to be recognized")
	}
}

func writeChannelSkillSpotlightFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context. CHANNEL_SKILL_SPOTLIGHT_DESCRIPTION_SECRET.
---
Full skill body secret CHANNEL_SKILL_SPOTLIGHT_BODY_SECRET.
`)
	writeTestFile(t, root, ".gitclaw/SKILLS/weekly-review/SKILL.md", `---
name: weekly-review
description: Summarize weekly planning notes.
---
Full skill body secret CHANNEL_SKILL_SPOTLIGHT_OTHER_BODY_SECRET.
`)
}
