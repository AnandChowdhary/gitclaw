package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRenderSessionCatalogReportListsBodyFreeSessionSurface(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 88,
			Title:  "@gitclaw /session catalog",
			Body:   "SESSION_CATALOG_BODY_SECRET",
		},
	}
	comments := []Comment{{
		ID:   21,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nSESSION_CATALOG_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "SESSION_CATALOG_TRANSCRIPT_SECRET"},
		{Role: "assistant", Body: "SESSION_CATALOG_ASSISTANT_SECRET"},
	}

	body := RenderSessionReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Session Catalog Report",
		"requested_session_command: `catalog`",
		"session_command_status: `ok`",
		"session_catalog_status: `ok`",
		"catalog_strategy: `compact-issue-thread-session-discovery`",
		"session_model: `github-issue-thread-plus-backup-json`",
		"canonical_session_store: `github-issue-thread`",
		"local_backup_store: `gitclaw-backups issue JSON`",
		"catalog_entries: `14`",
		"issue_side_commands: `14`",
		"local_backup_commands: `13`",
		"raw_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"repository_mutation_allowed: `false`",
		"session_deletion_allowed: `false`",
		"session_export_allowed_issue_side: `false`",
		"llm_e2e_required_after_session_catalog_change: `true`",
		"command=`catalog` issue_intent=`@gitclaw /session catalog` local_command=`gitclaw session catalog` execution=`metadata-only` gate=`body-free-output` raw_bodies_included=`false` mutation_allowed=`false`",
		"command=`provenance` issue_intent=`@gitclaw /session provenance` local_command=`gitclaw session provenance --backup <issue.json>` execution=`current-issue-or-local-backup` gate=`assistant-turn-marker-prompt-context`",
		"command=`tools` issue_intent=`@gitclaw /session tools` local_command=`gitclaw session tools --backup <issue.json>` execution=`current-issue-or-local-backup` gate=`assistant-turn-marker-tool-context`",
		"command=`skills` issue_intent=`@gitclaw /session skills` local_command=`gitclaw session skills --backup <issue.json>` execution=`current-issue-or-local-backup` gate=`assistant-turn-marker-skill-context`",
		"command=`usage` issue_intent=`@gitclaw /session usage` local_command=`gitclaw session usage --backup <issue.json>` execution=`current-issue-or-local-backup` gate=`assistant-turn-marker-token-telemetry`",
		"command=`trajectory` issue_intent=`@gitclaw /session trajectory` local_command=`gitclaw session trajectory --backup <issue.json>` execution=`current-issue-or-local-backup` gate=`body-free-assistant-turn-manifest`",
		"command=`compaction` issue_intent=`@gitclaw /session compaction` local_command=`gitclaw session compaction --backup <issue.json>` execution=`current-issue-or-local-backup` gate=`body-free-session-compaction-readiness`",
		"command=`resume` issue_intent=`@gitclaw /session resume` local_command=`gitclaw session resume --backup <issue.json>` execution=`current-issue-or-local-backup` gate=`github-issue-comment-continuation-readiness`",
		"command=`search` issue_intent=`@gitclaw /session search <query>` local_command=`gitclaw session search <query> --backup <issue.json>`",
		"issue_thread_gate=`canonical-session-is-github-issue-thread`",
		"provenance_gate=`assistant-turn-marker-prompt-context`",
		"tools_gate=`assistant-turn-marker-tool-context`",
		"skills_gate=`assistant-turn-marker-skill-context`",
		"usage_gate=`assistant-turn-marker-token-telemetry`",
		"trajectory_gate=`body-free-assistant-turn-manifest`",
		"compaction_gate=`body-free-session-compaction-readiness`",
		"resume_gate=`github-issue-comment-continuation-readiness`",
		"search_gate=`query-hash-and-line-hash-metadata`",
		"coverage_gate=`prompt-provenance-skill-tool-telemetry`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session catalog report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_CATALOG_BODY_SECRET", "SESSION_CATALOG_COMMENT_SECRET", "SESSION_CATALOG_TRANSCRIPT_SECRET", "SESSION_CATALOG_ASSISTANT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session catalog report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionSearchReportFindsTranscriptWithoutBodies(t *testing.T) {
	transcript := []TranscriptMessage{
		{
			Role:              "user",
			Body:              "Please remember deployment window SESSION_SEARCH_ISSUE_SECRET.",
			Actor:             "alice",
			AuthorAssociation: "MEMBER",
			Trusted:           true,
		},
		{
			Role:              "assistant",
			Body:              "Deployment window noted with SESSION_SEARCH_ASSISTANT_SECRET.",
			Actor:             "github-actions[bot]",
			AuthorAssociation: "NONE",
			CommentID:         42,
			Trusted:           true,
		},
	}
	body := RenderSessionSearchReport(Event{}, transcript, "deployment SESSION_SEARCH_QUERY_SECRET", 1)
	for _, want := range []string{
		"GitClaw Session Search Report",
		"scope: `local-cli`",
		"session_search_status: `ok`",
		"query_sha256_12:",
		"query_terms:",
		"max_results: `1`",
		"transcript_messages: `2`",
		"matched_messages: `2`",
		"matched_lines: `2`",
		"results_returned: `1`",
		"raw_bodies_included: `false`",
		"message=`01`",
		"role=`user`",
		"source=`issue`",
		"trusted=`true`",
		"message_sha256_12=",
		"line_sha256_12=",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session search report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_SEARCH_ISSUE_SECRET", "SESSION_SEARCH_ASSISTANT_SECRET", "SESSION_SEARCH_QUERY_SECRET", "deployment SESSION_SEARCH_QUERY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session search report leaked body/query token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportShowsAssistantTurnProvenanceWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:                51,
		Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"2\" skills=\"repo-reader\" tools=\"gitclaw.search_files,gitclaw.read_file\" -->\nSESSION_PROVENANCE_ASSISTANT_SECRET",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}
	transcript := []TranscriptMessage{{
		Role:      "assistant",
		Body:      "SESSION_PROVENANCE_ASSISTANT_SECRET",
		Actor:     "github-actions[bot]",
		CommentID: 51,
		Trusted:   true,
	}}
	body := renderSessionReport(Event{Repo: "owner/repo", Issue: Issue{Number: 5}}, comments, transcript, true, "")
	for _, want := range []string{
		"assistant_turns_with_prompt_provenance: `1`",
		"assistant_turns_missing_prompt_provenance: `0`",
		"unique_prompt_context_hashes: `1`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files, gitclaw.read_file`",
		"### Assistant Turn Provenance",
		"source=`comment:51`",
		"model=`openai/gpt-4.1-nano`",
		"prompt_context_sha256_12=`abc123abc123`",
		"context_documents=`2`",
		"selected_skills=`1`",
		"tool_outputs=`2`",
		"skills=`repo-reader`",
		"tools=`gitclaw.search_files, gitclaw.read_file`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SESSION_PROVENANCE_ASSISTANT_SECRET") {
		t.Fatalf("session report leaked assistant body:\n%s", body)
	}
}

func TestRenderSessionProvenanceReportShowsPromptEvidenceWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:                53,
		Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"2\" skills=\"repo-reader\" tools=\"gitclaw.search_files,gitclaw.read_file\" usage_prompt_tokens=\"100\" usage_completion_tokens=\"9\" usage_total_tokens=\"109\" usage_cache_read_tokens=\"7\" usage_cache_write_tokens=\"0\" -->\nSESSION_PROVENANCE_NAMED_ASSISTANT_SECRET",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}
	transcript := []TranscriptMessage{{
		Role:      "assistant",
		Body:      "SESSION_PROVENANCE_NAMED_ASSISTANT_SECRET",
		Actor:     "github-actions[bot]",
		CommentID: 53,
		Trusted:   true,
	}}
	body := RenderSessionProvenanceReport(BuildSessionProvenanceReport("issue-thread", "", Event{Repo: "owner/repo", Kind: "issue_comment", Issue: Issue{Number: 6}}, comments, transcript))
	for _, want := range []string{
		"GitClaw Session Provenance Report",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#6`",
		"event_kind: `issue_comment`",
		"session_provenance_status: `ok`",
		"provenance_scope: `assistant-turn-marker-prompt-context`",
		"session_store: `github-issue-thread`",
		"raw_comments: `1`",
		"transcript_messages: `1`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"assistant_turns_missing_prompt_provenance: `0`",
		"unique_prompt_context_hashes: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"model_names: `openai/gpt-4.1-nano`",
		"prompt_visible_skill_count: `1`",
		"prompt_visible_tool_count: `2`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files, gitclaw.read_file`",
		"usage_prompt_tokens: `100`",
		"usage_completion_tokens: `9`",
		"usage_total_tokens: `109`",
		"usage_cache_read_tokens: `7`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_session_provenance_change: `true`",
		"### Assistant Turn Provenance",
		"source=`comment:53`",
		"prompt_context_sha256_12=`abc123abc123`",
		"usage_present=`true`",
		"usage_total_tokens=`109`",
		"### Provenance Gates",
		"prompt_provenance_gate=`pass`",
		"model_backed_gate=`pass`",
		"skill_tool_gate=`pass`",
		"usage_telemetry_gate=`pass`",
		"raw_body_gate=`hashes-and-marker-attributes-only`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session provenance report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SESSION_PROVENANCE_NAMED_ASSISTANT_SECRET") {
		t.Fatalf("session provenance report leaked assistant body:\n%s", body)
	}
}

func TestRenderSessionReportRoutesProvenanceCommandWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:   54,
		Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" usage_total_tokens=\"12\" -->\nSESSION_PROVENANCE_ROUTE_SECRET",
	}}
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Kind:  "issue_comment",
		Issue: Issue{Number: 7, Title: "@gitclaw /session provenance", Body: "SESSION_PROVENANCE_ROUTE_ISSUE_SECRET"},
	}, DefaultConfig(), comments, nil)
	for _, want := range []string{"GitClaw Session Provenance Report", "session_provenance_status: `ok`", "provenance_scope: `assistant-turn-marker-prompt-context`", "prompt_visible_skill_names: `repo-reader`", "prompt_visible_tool_names: `gitclaw.search_files`", "usage_total_tokens: `12`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session provenance route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_PROVENANCE_ROUTE_SECRET", "SESSION_PROVENANCE_ROUTE_ISSUE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session provenance route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionToolsReportShowsToolLedgerWithoutBodies(t *testing.T) {
	comments := []Comment{
		{
			ID:                55,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.search_files\" usage_prompt_tokens=\"100\" usage_completion_tokens=\"9\" usage_total_tokens=\"109\" -->\nSESSION_TOOLS_MODEL_ASSISTANT_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
		{
			ID:                56,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-2\" model=\"gitclaw/session\" prompt_context_sha256_12=\"def456def456\" context_documents=\"1\" selected_skills=\"0\" tool_outputs=\"1\" tools=\"gitclaw.search_files\" -->\nSESSION_TOOLS_DETERMINISTIC_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
	}
	transcript := []TranscriptMessage{
		{Role: "assistant", Body: "SESSION_TOOLS_MODEL_ASSISTANT_SECRET", Actor: "github-actions[bot]", CommentID: 55, Trusted: true},
		{Role: "assistant", Body: "SESSION_TOOLS_DETERMINISTIC_SECRET", Actor: "github-actions[bot]", CommentID: 56, Trusted: true},
	}
	body := RenderSessionToolsReport(BuildSessionToolsReport("issue-thread", "", Event{Repo: "owner/repo", Kind: "issue_comment", Issue: Issue{Number: 8}}, comments, transcript))
	for _, want := range []string{
		"GitClaw Session Tools Report",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#8`",
		"event_kind: `issue_comment`",
		"session_tools_status: `ok`",
		"provenance_scope: `assistant-turn-marker-tool-context`",
		"session_store: `github-issue-thread`",
		"raw_comments: `2`",
		"transcript_messages: `2`",
		"assistant_turn_comments: `2`",
		"tool_backed_assistant_turns: `2`",
		"assistant_turns_missing_tool_context: `0`",
		"unique_prompt_visible_tools: `2`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`",
		"prompt_visible_tool_output_markers: `4`",
		"model_backed_tool_turns: `1`",
		"deterministic_tool_turns: `1`",
		"model_names: `gitclaw/session, openai/gpt-4.1-nano`",
		"usage_total_tokens: `109`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_session_tools_change: `true`",
		"### Tool Ledger",
		"tool=`gitclaw.search_files` prompt_visible_turns=`2` model_backed_turns=`1` deterministic_turns=`1` first_source=`comment:55` last_source=`comment:56` models=`gitclaw/session, openai/gpt-4.1-nano` prompt_context_hashes=`2`",
		"tool=`gitclaw.list_files` prompt_visible_turns=`1` model_backed_turns=`1` deterministic_turns=`0` first_source=`comment:55` last_source=`comment:55` models=`openai/gpt-4.1-nano` prompt_context_hashes=`1`",
		"### Tool Turn Evidence",
		"source=`comment:55` model=`openai/gpt-4.1-nano` prompt_context_sha256_12=`abc123abc123` tool_outputs=`3` tools=`gitclaw.list_files, gitclaw.search_files` usage_present=`true` usage_total_tokens=`109`",
		"source=`comment:56` model=`gitclaw/session` prompt_context_sha256_12=`def456def456` tool_outputs=`1` tools=`gitclaw.search_files` usage_present=`false` usage_total_tokens=`0`",
		"### Tool Gates",
		"tool_context_gate=`pass`",
		"model_backed_tool_gate=`pass`",
		"usage_telemetry_gate=`pass`",
		"raw_tool_input_gate=`marker-attributes-only`",
		"raw_tool_output_gate=`marker-attributes-only`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session tools report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_TOOLS_MODEL_ASSISTANT_SECRET", "SESSION_TOOLS_DETERMINISTIC_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session tools report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportRoutesToolsCommandWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:   57,
		Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" usage_total_tokens=\"12\" -->\nSESSION_TOOLS_ROUTE_SECRET",
	}}
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Kind:  "issue_comment",
		Issue: Issue{Number: 9, Title: "@gitclaw /session tools", Body: "SESSION_TOOLS_ROUTE_ISSUE_SECRET"},
	}, DefaultConfig(), comments, nil)
	for _, want := range []string{"GitClaw Session Tools Report", "session_tools_status: `ok`", "provenance_scope: `assistant-turn-marker-tool-context`", "tool_backed_assistant_turns: `1`", "prompt_visible_tool_names: `gitclaw.search_files`", "usage_total_tokens: `12`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session tools route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_TOOLS_ROUTE_SECRET", "SESSION_TOOLS_ROUTE_ISSUE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session tools route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionSkillsReportShowsSkillLedgerWithoutBodies(t *testing.T) {
	comments := []Comment{
		{
			ID:                58,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.search_files\" usage_prompt_tokens=\"100\" usage_completion_tokens=\"9\" usage_total_tokens=\"109\" -->\nSESSION_SKILLS_MODEL_ASSISTANT_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
		{
			ID:                59,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-2\" model=\"gitclaw/session\" prompt_context_sha256_12=\"def456def456\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"0\" skills=\"repo-reader\" -->\nSESSION_SKILLS_DETERMINISTIC_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
	}
	transcript := []TranscriptMessage{
		{Role: "assistant", Body: "SESSION_SKILLS_MODEL_ASSISTANT_SECRET", Actor: "github-actions[bot]", CommentID: 58, Trusted: true},
		{Role: "assistant", Body: "SESSION_SKILLS_DETERMINISTIC_SECRET", Actor: "github-actions[bot]", CommentID: 59, Trusted: true},
	}
	body := RenderSessionSkillsReport(BuildSessionSkillsReport("issue-thread", "", Event{Repo: "owner/repo", Kind: "issue_comment", Issue: Issue{Number: 10}}, comments, transcript))
	for _, want := range []string{
		"GitClaw Session Skills Report",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#10`",
		"event_kind: `issue_comment`",
		"session_skills_status: `ok`",
		"provenance_scope: `assistant-turn-marker-skill-context`",
		"session_store: `github-issue-thread`",
		"raw_comments: `2`",
		"transcript_messages: `2`",
		"assistant_turn_comments: `2`",
		"skill_backed_assistant_turns: `2`",
		"assistant_turns_missing_skill_context: `0`",
		"unique_prompt_visible_skills: `1`",
		"prompt_visible_skill_names: `repo-reader`",
		"selected_skill_markers: `2`",
		"model_backed_skill_turns: `1`",
		"deterministic_skill_turns: `1`",
		"model_names: `gitclaw/session, openai/gpt-4.1-nano`",
		"usage_total_tokens: `109`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_session_skills_change: `true`",
		"### Skill Ledger",
		"skill=`repo-reader` prompt_visible_turns=`2` model_backed_turns=`1` deterministic_turns=`1` first_source=`comment:58` last_source=`comment:59` models=`gitclaw/session, openai/gpt-4.1-nano` prompt_context_hashes=`2`",
		"### Skill Turn Evidence",
		"source=`comment:58` model=`openai/gpt-4.1-nano` prompt_context_sha256_12=`abc123abc123` selected_skills=`1` skills=`repo-reader` usage_present=`true` usage_total_tokens=`109`",
		"source=`comment:59` model=`gitclaw/session` prompt_context_sha256_12=`def456def456` selected_skills=`1` skills=`repo-reader` usage_present=`false` usage_total_tokens=`0`",
		"### Skill Gates",
		"skill_context_gate=`pass`",
		"model_backed_skill_gate=`pass`",
		"usage_telemetry_gate=`pass`",
		"raw_skill_body_gate=`marker-attributes-only`",
		"raw_tool_output_gate=`marker-attributes-only`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session skills report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_SKILLS_MODEL_ASSISTANT_SECRET", "SESSION_SKILLS_DETERMINISTIC_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session skills report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportRoutesSkillsCommandWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:   60,
		Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" usage_total_tokens=\"12\" -->\nSESSION_SKILLS_ROUTE_SECRET",
	}}
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Kind:  "issue_comment",
		Issue: Issue{Number: 11, Title: "@gitclaw /session skills", Body: "SESSION_SKILLS_ROUTE_ISSUE_SECRET"},
	}, DefaultConfig(), comments, nil)
	for _, want := range []string{"GitClaw Session Skills Report", "session_skills_status: `ok`", "provenance_scope: `assistant-turn-marker-skill-context`", "skill_backed_assistant_turns: `1`", "prompt_visible_skill_names: `repo-reader`", "usage_total_tokens: `12`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session skills route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_SKILLS_ROUTE_SECRET", "SESSION_SKILLS_ROUTE_ISSUE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session skills route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionUsageReportShowsUsageLedgerWithoutBodies(t *testing.T) {
	comments := []Comment{
		{
			ID:                61,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.search_files\" usage_prompt_tokens=\"100\" usage_completion_tokens=\"9\" usage_total_tokens=\"109\" usage_cache_read_tokens=\"7\" usage_cache_write_tokens=\"2\" -->\nSESSION_USAGE_MODEL_ASSISTANT_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
		{
			ID:                62,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-2\" model=\"gitclaw/session\" prompt_context_sha256_12=\"def456def456\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"0\" skills=\"repo-reader\" usage_prompt_tokens=\"4\" usage_completion_tokens=\"1\" usage_total_tokens=\"5\" -->\nSESSION_USAGE_DETERMINISTIC_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
	}
	transcript := []TranscriptMessage{
		{Role: "assistant", Body: "SESSION_USAGE_MODEL_ASSISTANT_SECRET", Actor: "github-actions[bot]", CommentID: 61, Trusted: true},
		{Role: "assistant", Body: "SESSION_USAGE_DETERMINISTIC_SECRET", Actor: "github-actions[bot]", CommentID: 62, Trusted: true},
	}
	body := RenderSessionUsageReport(BuildSessionUsageReport("issue-thread", "", Event{Repo: "owner/repo", Kind: "issue_comment", Issue: Issue{Number: 12}}, comments, transcript))
	for _, want := range []string{
		"GitClaw Session Usage Report",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#12`",
		"event_kind: `issue_comment`",
		"session_usage_status: `ok`",
		"usage_scope: `assistant-turn-marker-token-telemetry`",
		"session_store: `github-issue-thread`",
		"raw_comments: `2`",
		"transcript_messages: `2`",
		"assistant_turn_comments: `2`",
		"usage_bearing_assistant_turns: `2`",
		"assistant_turns_missing_usage_telemetry: `0`",
		"model_backed_usage_turns: `1`",
		"deterministic_usage_turns: `1`",
		"model_names: `gitclaw/session, openai/gpt-4.1-nano`",
		"usage_prompt_tokens: `104`",
		"usage_completion_tokens: `10`",
		"usage_total_tokens: `114`",
		"usage_cache_read_tokens: `7`",
		"usage_cache_write_tokens: `2`",
		"latest_usage_source: `comment:62`",
		"latest_usage_model: `gitclaw/session`",
		"latest_usage_total_tokens: `5`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_provider_usage_included: `false`",
		"raw_provider_responses_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_session_usage_change: `true`",
		"### Usage Ledger",
		"model=`gitclaw/session` assistant_turns=`1` usage_turns=`1` model_backed_turns=`0` deterministic_turns=`1` prompt_tokens=`4` completion_tokens=`1` total_tokens=`5`",
		"model=`openai/gpt-4.1-nano` assistant_turns=`1` usage_turns=`1` model_backed_turns=`1` deterministic_turns=`0` prompt_tokens=`100` completion_tokens=`9` total_tokens=`109` cache_read_tokens=`7` cache_write_tokens=`2`",
		"### Usage Turn Evidence",
		"source=`comment:61` model=`openai/gpt-4.1-nano` prompt_context_sha256_12=`abc123abc123` usage_present=`true` usage_prompt_tokens=`100` usage_completion_tokens=`9` usage_total_tokens=`109` usage_cache_read_tokens=`7` usage_cache_write_tokens=`2`",
		"source=`comment:62` model=`gitclaw/session` prompt_context_sha256_12=`def456def456` usage_present=`true` usage_prompt_tokens=`4` usage_completion_tokens=`1` usage_total_tokens=`5` usage_cache_read_tokens=`0` usage_cache_write_tokens=`0`",
		"### Usage Gates",
		"usage_telemetry_gate=`pass`",
		"model_backed_usage_gate=`pass`",
		"raw_provider_usage_gate=`marker-attributes-only`",
		"raw_provider_response_gate=`disabled`",
		"raw_prompt_body_gate=`hashes-and-marker-attributes-only`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session usage report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_USAGE_MODEL_ASSISTANT_SECRET", "SESSION_USAGE_DETERMINISTIC_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session usage report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportRoutesUsageCommandWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:   63,
		Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" usage_prompt_tokens=\"10\" usage_completion_tokens=\"2\" usage_total_tokens=\"12\" -->\nSESSION_USAGE_ROUTE_SECRET",
	}}
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Kind:  "issue_comment",
		Issue: Issue{Number: 12, Title: "@gitclaw /session usage", Body: "SESSION_USAGE_ROUTE_ISSUE_SECRET"},
	}, DefaultConfig(), comments, nil)
	for _, want := range []string{"GitClaw Session Usage Report", "session_usage_status: `ok`", "usage_scope: `assistant-turn-marker-token-telemetry`", "usage_bearing_assistant_turns: `1`", "model_backed_usage_turns: `1`", "usage_total_tokens: `12`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session usage route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_USAGE_ROUTE_SECRET", "SESSION_USAGE_ROUTE_ISSUE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session usage route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionTrajectoryReportShowsBodyFreeManifest(t *testing.T) {
	comments := []Comment{
		{
			ID:                64,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" event_id=\"issue-12\" model=\"openai/gpt-4.1-nano\" idempotency_key=\"idem-1\" run_url=\"https://github.com/owner/repo/actions/runs/123\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.search_files\" usage_prompt_tokens=\"100\" usage_completion_tokens=\"9\" usage_total_tokens=\"109\" usage_cache_read_tokens=\"7\" usage_cache_write_tokens=\"2\" -->\nSESSION_TRAJECTORY_MODEL_ASSISTANT_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
		{
			ID:                65,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-2\" event_id=\"comment-2\" model=\"gitclaw/session\" idempotency_key=\"idem-2\" run_url=\"https://github.com/owner/repo/actions/runs/124\" prompt_context_sha256_12=\"def456def456\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"0\" skills=\"repo-reader\" usage_total_tokens=\"5\" -->\nSESSION_TRAJECTORY_DETERMINISTIC_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
	}
	transcript := []TranscriptMessage{
		{Role: "assistant", Body: "SESSION_TRAJECTORY_MODEL_ASSISTANT_SECRET", Actor: "github-actions[bot]", CommentID: 64, Trusted: true},
		{Role: "assistant", Body: "SESSION_TRAJECTORY_DETERMINISTIC_SECRET", Actor: "github-actions[bot]", CommentID: 65, Trusted: true},
	}
	body := RenderSessionTrajectoryReport(BuildSessionTrajectoryReport("issue-thread", "", Event{Repo: "owner/repo", Kind: "issue_comment", Issue: Issue{Number: 13}}, comments, transcript))
	for _, want := range []string{
		"GitClaw Session Trajectory Report",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#13`",
		"event_kind: `issue_comment`",
		"session_trajectory_status: `ok`",
		"trajectory_scope: `body-free-assistant-turn-manifest`",
		"export_format: `gitclaw.session-trajectory.v1`",
		"session_store: `github-issue-thread`",
		"raw_comments: `2`",
		"transcript_messages: `2`",
		"assistant_turn_comments: `2`",
		"trajectory_turns: `2`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `1`",
		"model_names: `gitclaw/session, openai/gpt-4.1-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`",
		"unique_prompt_context_hashes: `2`",
		"run_metadata_turns: `2`",
		"unique_run_id_hashes: `2`",
		"context_documents_total: `3`",
		"selected_skills_total: `2`",
		"tool_outputs_total: `3`",
		"usage_bearing_assistant_turns: `2`",
		"usage_prompt_tokens: `100`",
		"usage_completion_tokens: `9`",
		"usage_total_tokens: `114`",
		"usage_cache_read_tokens: `7`",
		"usage_cache_write_tokens: `2`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_provider_responses_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_session_trajectory_change: `true`",
		"### Trajectory Manifest",
		"turn=`01` source=`comment:64` model=`openai/gpt-4.1-nano` deterministic=`false`",
		"prompt_context_sha256_12=`abc123abc123` context_documents=`2` selected_skills=`1` tool_outputs=`3` skills=`repo-reader` tools=`gitclaw.list_files, gitclaw.search_files` usage_present=`true` usage_prompt_tokens=`100` usage_completion_tokens=`9` usage_total_tokens=`109`",
		"turn=`02` source=`comment:65` model=`gitclaw/session` deterministic=`true`",
		"prompt_context_sha256_12=`def456def456` context_documents=`1` selected_skills=`1` tool_outputs=`0` skills=`repo-reader` tools=`none` usage_present=`true`",
		"run_id_sha256_12=",
		"event_id_sha256_12=",
		"idempotency_key_sha256_12=",
		"run_url_sha256_12=",
		"assistant_comment_sha256_12=",
		"prompt_provenance_gate=`pass`",
		"model_backed_gate=`pass`",
		"run_metadata_gate=`pass`",
		"usage_telemetry_gate=`pass`",
		"raw_body_gate=`hashes-and-marker-attributes-only`",
		"raw_provider_response_gate=`disabled`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session trajectory report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_TRAJECTORY_MODEL_ASSISTANT_SECRET", "SESSION_TRAJECTORY_DETERMINISTIC_SECRET", "https://github.com/owner/repo/actions/runs/123", "run-1", "idem-1"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session trajectory report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportRoutesTrajectoryCommandWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:   66,
		Body: "<!-- gitclaw:assistant-turn run_id=\"run-1\" event_id=\"issue-13\" model=\"openai/gpt-4.1-nano\" idempotency_key=\"idem-1\" run_url=\"https://github.com/owner/repo/actions/runs/123\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" usage_prompt_tokens=\"10\" usage_completion_tokens=\"2\" usage_total_tokens=\"12\" -->\nSESSION_TRAJECTORY_ROUTE_SECRET",
	}}
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Kind:  "issue_comment",
		Issue: Issue{Number: 13, Title: "@gitclaw /session trajectory", Body: "SESSION_TRAJECTORY_ROUTE_ISSUE_SECRET"},
	}, DefaultConfig(), comments, nil)
	for _, want := range []string{"GitClaw Session Trajectory Report", "session_trajectory_status: `ok`", "trajectory_scope: `body-free-assistant-turn-manifest`", "trajectory_turns: `1`", "model_backed_assistant_turns: `1`", "usage_total_tokens: `12`", "run_metadata_gate=`pass`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session trajectory route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_TRAJECTORY_ROUTE_SECRET", "SESSION_TRAJECTORY_ROUTE_ISSUE_SECRET", "https://github.com/owner/repo/actions/runs/123"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session trajectory route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionCompactionReportShowsReadinessWithoutBodies(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxPromptBytes = 400
	cfg.MaxTranscriptMessages = 3
	cfg.MaxTranscriptMessageBytes = 45
	comments := []Comment{{
		ID:                67,
		Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"2\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" usage_prompt_tokens=\"100\" usage_completion_tokens=\"9\" usage_total_tokens=\"109\" usage_cache_read_tokens=\"7\" usage_cache_write_tokens=\"2\" -->\nSESSION_COMPACTION_MODEL_ASSISTANT_SECRET",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "SESSION_COMPACTION_ISSUE_SECRET opening context that is deliberately long enough", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
		{Role: "assistant", Body: "SESSION_COMPACTION_MODEL_ASSISTANT_SECRET first answer with long text", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 67, Trusted: true},
		{Role: "user", Body: "SESSION_COMPACTION_COMMENT_SECRET follow up body", Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 68, Trusted: true},
		{Role: "assistant", Body: "SESSION_COMPACTION_DETERMINISTIC_SECRET session report", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 69, Trusted: true},
	}
	body := RenderSessionCompactionReport(BuildSessionCompactionReport("issue-thread", "", Event{Repo: "owner/repo", Kind: "issue_comment", Issue: Issue{Number: 14}}, cfg, comments, transcript))
	for _, want := range []string{
		"GitClaw Session Compaction Report",
		"Generated without a model call",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#14`",
		"event_kind: `issue_comment`",
		"session_compaction_status: `warn`",
		"compaction_scope: `body-free-session-compaction-readiness`",
		"compaction_strategy: `github-issue-thread-body-free-compaction-readiness`",
		"compression_model: `hermes-dual-thresholds+openclaw-trajectory-pruning`",
		"session_store: `github-issue-thread`",
		"raw_comments: `1`",
		"transcript_messages: `4`",
		"user_messages: `2`",
		"assistant_messages: `2`",
		"max_prompt_bytes: `400`",
		"max_transcript_messages: `3`",
		"max_transcript_message_bytes: `45`",
		"bounded_transcript_messages: `3`",
		"omitted_older_messages: `1`",
		"agent_compression_threshold_percent: `50`",
		"agent_compression_threshold_bytes: `200`",
		"gateway_hygiene_threshold_percent: `85`",
		"gateway_hygiene_threshold_bytes: `340`",
		"agent_compaction_recommended: `true`",
		"gateway_hygiene_recommended: `false`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"unique_prompt_context_hashes: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"model_names: `openai/gpt-4.1-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files`",
		"usage_bearing_assistant_turns: `1`",
		"usage_prompt_tokens: `100`",
		"usage_completion_tokens: `9`",
		"usage_total_tokens: `109`",
		"usage_cache_read_tokens: `7`",
		"usage_cache_write_tokens: `2`",
		"lossy_summary_supported: `false`",
		"lossless_session_search_supported: `true`",
		"issue_thread_canonical_storage: `true`",
		"backup_branch_replay_preferred: `true`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_provider_usage_included: `false`",
		"raw_provider_responses_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"repository_mutation_allowed: `false`",
		"compaction_mutation_allowed: `false`",
		"compression_writes_memory_allowed: `false`",
		"llm_e2e_required_after_session_compaction_change: `true`",
		"### Compaction Cards",
		"message=`01` role=`user`",
		"compaction_region=`session-anchor`",
		"message=`02` role=`assistant`",
		"omitted_by_limit=`true`",
		"compaction_action=`keep-in-issue-and-backup-search`",
		"message=`04` role=`assistant`",
		"compaction_region=`latest-turn`",
		"### Compaction Gates",
		"agent_compaction_gate=`warn`",
		"gateway_hygiene_gate=`pass`",
		"model_backed_gate=`pass`",
		"lossless_recall_gate=`backup-json-and-session-search`",
		"raw_body_gate=`hashes-counts-and-metadata-only`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session compaction report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_COMPACTION_ISSUE_SECRET", "SESSION_COMPACTION_MODEL_ASSISTANT_SECRET", "SESSION_COMPACTION_COMMENT_SECRET", "SESSION_COMPACTION_DETERMINISTIC_SECRET", "run-1"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session compaction report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportRoutesCompactionCommandWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:   70,
		Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" usage_prompt_tokens=\"10\" usage_completion_tokens=\"2\" usage_total_tokens=\"12\" -->\nSESSION_COMPACTION_ROUTE_SECRET",
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "SESSION_COMPACTION_ROUTE_ISSUE_SECRET", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
		{Role: "assistant", Body: "SESSION_COMPACTION_ROUTE_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 70, Trusted: true},
	}
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Kind:  "issue_comment",
		Issue: Issue{Number: 14, Title: "@gitclaw /session compaction", Body: "SESSION_COMPACTION_ROUTE_ISSUE_SECRET"},
	}, DefaultConfig(), comments, transcript)
	for _, want := range []string{"GitClaw Session Compaction Report", "session_compaction_status: `ok`", "compaction_scope: `body-free-session-compaction-readiness`", "model_backed_assistant_turns: `1`", "usage_total_tokens: `12`", "agent_compaction_gate=`pass`", "gateway_hygiene_gate=`pass`", "model_backed_gate=`pass`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session compaction route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_COMPACTION_ROUTE_SECRET", "SESSION_COMPACTION_ROUTE_ISSUE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session compaction route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestBuildSessionPromptProvenanceReportCountsMissingMarkers(t *testing.T) {
	report := buildSessionPromptProvenanceReport([]Comment{{
		ID:   52,
		Body: "<!-- gitclaw:assistant-turn model=\"gitclaw/context\" -->\nold deterministic report",
	}})
	if report.TurnsWithProvenance != 0 || report.PromptContextHashMissing != 1 || len(report.Turns) != 1 {
		t.Fatalf("unexpected provenance report: %#v", report)
	}
}

func TestRenderSessionCoverageReportRequiresModelBackedProvenanceWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:                61,
		Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.search_files\" -->\nSESSION_COVERAGE_ASSISTANT_SECRET",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}
	transcript := []TranscriptMessage{{
		Role:      "assistant",
		Body:      "SESSION_COVERAGE_ASSISTANT_SECRET",
		Actor:     "github-actions[bot]",
		CommentID: 61,
		Trusted:   true,
	}}
	req := DefaultSessionCoverageRequirements()
	req.RequiredSkills = []string{"repo-reader"}
	req.RequiredTools = []string{"gitclaw.search_files"}
	report := BuildSessionCoverageReport("issue-thread", "", Event{Repo: "owner/repo", Issue: Issue{Number: 5}}, comments, transcript, req)
	if !report.OK() {
		t.Fatalf("session coverage unexpectedly failed: %#v", report)
	}
	body := RenderSessionCoverageReport(report)
	for _, want := range []string{
		"GitClaw Session Coverage Report",
		"session_coverage_status: `ok`",
		"required_assistant_turns: `1`",
		"required_prompt_provenance_turns: `1`",
		"required_model_backed_turns: `1`",
		"required_skill_names: `repo-reader`",
		"required_tool_names: `gitclaw.search_files`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"model_names: `openai/gpt-4.1-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`",
		"missing_required_skill_names: `none`",
		"missing_required_tool_names: `none`",
		"raw_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"llm_e2e_required_after_session_coverage_change: `true`",
		"assistant_turns_met=`true`",
		"prompt_provenance_met=`true`",
		"model_backed_turns_met=`true`",
		"required_skills_met=`true`",
		"required_tools_met=`true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session coverage report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SESSION_COVERAGE_ASSISTANT_SECRET") {
		t.Fatalf("session coverage report leaked assistant body:\n%s", body)
	}
}

func TestRenderSessionCoverageReportWarnsWithoutModelBackedTurn(t *testing.T) {
	report := BuildSessionCoverageReport("issue-thread", "", Event{Repo: "owner/repo", Issue: Issue{Number: 5}}, []Comment{{
		ID:   62,
		Body: "<!-- gitclaw:assistant-turn model=\"gitclaw/session\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"0\" tool_outputs=\"0\" -->\ndeterministic body",
	}}, nil, DefaultSessionCoverageRequirements())
	if report.OK() || report.SessionCoverageStatus != "warn" || report.ModelBackedTurnsMet {
		t.Fatalf("session coverage unexpectedly passed: %#v", report)
	}
	body := RenderSessionCoverageReport(report)
	for _, want := range []string{"session_coverage_status: `warn`", "model_backed_assistant_turns: `0`", "deterministic_assistant_turns: `1`", "model_backed_turns_met=`false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("warning session coverage report missing %q:\n%s", want, body)
		}
	}
}

func TestRenderSessionReportRoutesCoverageCommand(t *testing.T) {
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Issue: Issue{Number: 5, Title: "@gitclaw /session coverage"},
	}, DefaultConfig(), []Comment{{
		ID:   63,
		Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" -->\ncoverage body secret",
	}}, nil)
	for _, want := range []string{"GitClaw Session Coverage Report", "session_coverage_status: `ok`", "model_backed_assistant_turns: `1`", "prompt_visible_skill_names: `repo-reader`", "prompt_visible_tool_names: `gitclaw.search_files`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session coverage route missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "coverage body secret") {
		t.Fatalf("session coverage route leaked body:\n%s", body)
	}
}

func TestSessionCoverageCommandReportsBackupCoverageWithoutBodies(t *testing.T) {
	root := t.TempDir()
	backup := IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-31T00:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 22,
			Title:  "@gitclaw coverage backup SESSION_COVERAGE_BACKUP_TITLE_SECRET",
			Body:   "SESSION_COVERAGE_BACKUP_BODY_SECRET",
		},
		Comments: []IssueBackupComment{{
			ID:                71,
			Body:              "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"def456def456\" context_documents=\"4\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.skill_index,gitclaw.search_files\" -->\nSESSION_COVERAGE_BACKUP_ASSISTANT_SECRET",
			Author:            "github-actions[bot]",
			AuthorAssociation: "NONE",
		}},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "SESSION_COVERAGE_BACKUP_USER_SECRET", Actor: "alice", Trusted: true},
			{Role: "assistant", Body: "SESSION_COVERAGE_BACKUP_ASSISTANT_SECRET", Actor: "github-actions[bot]", CommentID: 71, Trusted: true},
		},
	}
	writeBackupFixture(t, root, backup)
	backupPath := issueBackupPath(root, backup.Repo, backup.Issue.Number)

	output := captureStdout(t, func() {
		err := RunCLI(context.Background(), []string{
			"session", "coverage",
			"--backup", backupPath,
			"--require-skill", "repo-reader",
			"--require-tool", "gitclaw.search_files",
		})
		if err != nil {
			t.Fatalf("session coverage returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Session Coverage Report",
		"scope: `local-backup`",
		"backup_file:",
		"repository: `owner/repo`",
		"issue: `#22`",
		"session_coverage_status: `ok`",
		"model_backed_assistant_turns: `1`",
		"required_skill_names: `repo-reader`",
		"required_tool_names: `gitclaw.search_files`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.skill_index, gitclaw.search_files`",
		"required_tools_met=`true`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("session coverage output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SESSION_COVERAGE_BACKUP_TITLE_SECRET", "SESSION_COVERAGE_BACKUP_BODY_SECRET", "SESSION_COVERAGE_BACKUP_USER_SECRET", "SESSION_COVERAGE_BACKUP_ASSISTANT_SECRET", "@gitclaw coverage backup"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("session coverage leaked body/title token %q:\n%s", leaked, output)
		}
	}
}
