package gitclaw

import (
	"fmt"
	"strings"
)

type approvalProvenanceMarker struct {
	Source            string
	Model             string
	ModelRecognized   bool
	ModelSHA          string
	Deterministic     bool
	HasPromptEvidence bool
	ContextDocuments  int
	SelectedSkills    int
	ToolOutputs       int
	RunIDSHA          string
	EventIDSHA        string
	IdempotencyKeySHA string
	RunURLSHA         string
	MarkerSHA         string
}

type approvalProvenanceFinding struct {
	Severity string
	Code     string
	Detail   string
}

func RenderApprovalProvenanceCLIReport(cfg Config) string {
	return renderApprovalProvenanceReport(Event{}, cfg, PreflightDecision{}, nil, nil, false, false)
}

func renderApprovalProvenanceReport(ev Event, cfg Config, decision PreflightDecision, comments []Comment, transcript []TranscriptMessage, writeRequested bool, includeIssue bool) string {
	approvedLabelPresent := false
	needsHumanLabelPresent := false
	writeRequestedLabelPresent := false
	writeRequestEvidencePresent := writeRequested
	disabledLabelPresent := false
	actorTrusted := false
	actor := "none"
	labels := []string{}
	if includeIssue {
		labels = sortedStrings(ev.Issue.Labels)
		approvedLabelPresent = hasLabel(ev.Issue.Labels, defaultApprovedLabel)
		needsHumanLabelPresent = hasLabel(ev.Issue.Labels, defaultNeedsHumanLabel)
		writeRequestedLabelPresent = hasLabel(ev.Issue.Labels, cfg.WriteRequestedLabel)
		writeRequestEvidencePresent = writeRequested || writeRequestedLabelPresent
		disabledLabelPresent = hasLabel(ev.Issue.Labels, cfg.DisabledLabel)
		actor = actorAssociation(ev)
		actorTrusted = trustedAssociation(actor, cfg)
	}
	markers := approvalProvenanceMarkers(comments, cfg)
	modelBacked, deterministic, unrecognized := approvalMarkerModelCounts(markers)
	labelEvidenceSHA := shortDocumentHash(strings.Join(labels, "\n"))
	activeCommand := strings.Join(activeSlashCommandFields(ev, cfg), " ")
	findings := approvalProvenanceFindings(includeIssue, writeRequested, approvedLabelPresent, writeRequestedLabelPresent, needsHumanLabelPresent, disabledLabelPresent, actorTrusted, markers)
	status := approvalProvenanceStatus(includeIssue, writeRequestEvidencePresent, approvedLabelPresent, actorTrusted, disabledLabelPresent, findings)

	var b strings.Builder
	b.WriteString("## GitClaw Approvals Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- active_command: `%s`\n", inlineCode(activeCommand))
		fmt.Fprintf(&b, "- preflight_allowed: `%t`\n", decision.Allowed)
		fmt.Fprintf(&b, "- preflight_code: `%s`\n", decision.Code)
		fmt.Fprintf(&b, "- actor_association: `%s`\n", actor)
		fmt.Fprintf(&b, "- actor_trusted: `%t`\n", actorTrusted)
		fmt.Fprintf(&b, "- triggered: `%t`\n", triggered(ev, cfg))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- active_command_sha256_12: `%s`\n", shortDocumentHash(activeCommand))
		fmt.Fprintf(&b, "- turn_idempotency_key_sha256_12: `%s`\n", shortDocumentHash(IdempotencyKey(ev)))
		fmt.Fprintf(&b, "- current_issue_labels_available: `%t`\n", true)
		fmt.Fprintf(&b, "- current_issue_labels: `%d`\n", len(labels))
		fmt.Fprintf(&b, "- managed_labels_present: `%d`\n", countPresentLabels(labels, managedPolicyLabels(cfg)))
		fmt.Fprintf(&b, "- unmanaged_labels_present: `%d`\n", unmanagedLabelCount(labels, managedPolicyLabels(cfg), approvalRiskLabels(cfg)))
		fmt.Fprintf(&b, "- current_issue_label_set_sha256_12: `%s`\n", labelEvidenceSHA)
		fmt.Fprintf(&b, "- disabled_label_present: `%t`\n", disabledLabelPresent)
		fmt.Fprintf(&b, "- write_request_detected: `%t`\n", writeRequested)
		fmt.Fprintf(&b, "- write_requested_label_present: `%t`\n", writeRequestedLabelPresent)
		fmt.Fprintf(&b, "- write_request_evidence_present: `%t`\n", writeRequestEvidencePresent)
		fmt.Fprintf(&b, "- approved_label_present: `%t`\n", approvedLabelPresent)
		fmt.Fprintf(&b, "- needs_human_label_present: `%t`\n", needsHumanLabelPresent)
		fmt.Fprintf(&b, "- comments_available: `%t`\n", true)
		fmt.Fprintf(&b, "- issue_comments: `%d`\n", len(comments))
		fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
		fmt.Fprintf(&b, "- user_messages: `%d`\n", transcriptRoleCount(transcript, "user"))
		fmt.Fprintf(&b, "- assistant_messages: `%d`\n", transcriptRoleCount(transcript, "assistant"))
		fmt.Fprintf(&b, "- assistant_turn_markers: `%d`\n", len(markers))
		fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", modelBacked)
		fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", deterministic)
		fmt.Fprintf(&b, "- unrecognized_assistant_turn_markers: `%d`\n", unrecognized)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
		fmt.Fprintf(&b, "- current_issue_labels_available: `%t`\n", false)
		fmt.Fprintf(&b, "- comments_available: `%t`\n", false)
	}
	fmt.Fprintf(&b, "- approval_provenance_status: `%s`\n", status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", "current-issue-labels-transcript-and-assistant-markers")
	fmt.Fprintf(&b, "- approval_status: `%s`\n", approvalStatus(includeIssue, writeRequestEvidencePresent, approvedLabelPresent, actorTrusted))
	fmt.Fprintf(&b, "- approval_decision: `%s`\n", approvalDecision(writeRequestEvidencePresent, approvedLabelPresent))
	fmt.Fprintf(&b, "- approval_store: `%s`\n", "github-issue-labels")
	fmt.Fprintf(&b, "- approval_scope: `%s`\n", "per-issue")
	fmt.Fprintf(&b, "- approval_label: `%s`\n", defaultApprovedLabel)
	fmt.Fprintf(&b, "- needs_human_label: `%s`\n", defaultNeedsHumanLabel)
	fmt.Fprintf(&b, "- write_requested_label: `%s`\n", cfg.WriteRequestedLabel)
	fmt.Fprintf(&b, "- label_source: `%s`\n", "current-github-issue-labels")
	fmt.Fprintf(&b, "- write_request_source: `%s`\n", "transcript-heuristic-or-label")
	fmt.Fprintf(&b, "- actor_source: `%s`\n", "github-event-author-association")
	fmt.Fprintf(&b, "- preflight_source: `%s`\n", "github-event-plus-repo-config")
	fmt.Fprintf(&b, "- assistant_marker_source: `%s`\n", "issue-comments")
	fmt.Fprintf(&b, "- write_actions_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- host_exec_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comments_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_approval_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- run_urls_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_approval_provenance_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This report explains the approval evidence chain GitClaw would rely on before future write-capable work. It hashes issue, label, transcript, and assistant-marker evidence without approving anything, enabling writes, executing commands, exposing run URLs, or printing issue bodies, comments, prompts, approval payloads, credentials, or secret values.\n\n")

	b.WriteString("### Provenance Chain\n")
	if includeIssue {
		fmt.Fprintf(&b, "- source=`github-event` actor_association=`%s` actor_trusted=`%t` preflight_allowed=`%t` preflight_code=`%s` evidence_sha256_12=`%s`\n", actor, actorTrusted, decision.Allowed, decision.Code, shortDocumentHash(actor+"|"+decision.Code))
		fmt.Fprintf(&b, "- source=`issue-labels` store=`github-issue-labels` scope=`per-issue` labels=`%d` managed_present=`%d` evidence_sha256_12=`%s`\n", len(labels), countPresentLabels(labels, managedPolicyLabels(cfg)), labelEvidenceSHA)
		fmt.Fprintf(&b, "- source=`transcript` write_request_detected=`%t` transcript_messages=`%d` user_messages=`%d` assistant_messages=`%d` evidence_sha256_12=`%s`\n", writeRequested, len(transcript), transcriptRoleCount(transcript, "user"), transcriptRoleCount(transcript, "assistant"), shortDocumentHash(transcriptShape(transcript)))
		fmt.Fprintf(&b, "- source=`assistant-markers` assistant_turn_markers=`%d` model_backed=`%d` deterministic=`%d` unrecognized=`%d` evidence_sha256_12=`%s`\n", len(markers), modelBacked, deterministic, unrecognized, approvalMarkersEvidenceHash(markers))
	} else {
		b.WriteString("- source=`local-config` store=`github-issue-labels` scope=`per-issue` current_issue_labels_available=`false`\n")
	}
	b.WriteString("- source=`runtime-boundary` write_actions_enabled=`false` repository_mutation_allowed=`false` host_exec_allowed=`false`\n")

	b.WriteString("\n### Managed Label Evidence\n")
	writeApprovalLabelEvidence(&b, cfg, labels, includeIssue)

	b.WriteString("\n### Assistant Marker Evidence\n")
	writeApprovalMarkerEvidence(&b, markers)

	b.WriteString("\n### Findings\n")
	writeApprovalProvenanceFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func isApprovalProvenanceRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 &&
		(fields[0] == "/approvals" || fields[0] == "/approval") &&
		(strings.EqualFold(fields[1], "provenance") || strings.EqualFold(fields[1], "trace") || strings.EqualFold(fields[1], "evidence"))
}

func approvalProvenanceMarkers(comments []Comment, cfg Config) []approvalProvenanceMarker {
	markers := make([]approvalProvenanceMarker, 0)
	for _, comment := range comments {
		match := markerPattern.FindStringSubmatch(comment.Body)
		if len(match) < 2 {
			continue
		}
		attrs := match[1]
		rawModel := markerAttribute(attrs, "model")
		model, modelRecognized := approvalMarkerModelEvidence(rawModel, cfg)
		markers = append(markers, approvalProvenanceMarker{
			Source:            fmt.Sprintf("comment:%d", comment.ID),
			Model:             model,
			ModelRecognized:   modelRecognized,
			ModelSHA:          shortDocumentHash(rawModel),
			Deterministic:     modelRecognized && strings.HasPrefix(model, "gitclaw/"),
			HasPromptEvidence: markerAttribute(attrs, "prompt_context_sha256_12") != "",
			ContextDocuments:  markerAttributeInt(attrs, "context_documents"),
			SelectedSkills:    markerAttributeInt(attrs, "selected_skills"),
			ToolOutputs:       markerAttributeInt(attrs, "tool_outputs"),
			RunIDSHA:          shortDocumentHash(markerAttribute(attrs, "run_id")),
			EventIDSHA:        shortDocumentHash(markerAttribute(attrs, "event_id")),
			IdempotencyKeySHA: shortDocumentHash(markerAttribute(attrs, "idempotency_key")),
			RunURLSHA:         shortDocumentHash(markerAttribute(attrs, "run_url")),
			MarkerSHA:         shortDocumentHash(match[0]),
		})
	}
	return markers
}

func approvalMarkerModelEvidence(model string, cfg Config) (string, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "none", false
	}
	allowed := map[string]bool{}
	if strings.TrimSpace(cfg.Model) != "" {
		allowed[strings.TrimSpace(cfg.Model)] = true
	}
	for _, fallback := range normalizeModelFallbacks(cfg.ModelFallbacks) {
		allowed[fallback] = true
	}
	for _, entry := range commandCatalog {
		allowed[entry.Model] = true
	}
	if allowed[model] {
		return model, true
	}
	return "unrecognized", false
}

func approvalMarkerModelCounts(markers []approvalProvenanceMarker) (modelBacked int, deterministic int, unrecognized int) {
	for _, marker := range markers {
		if marker.Deterministic {
			deterministic++
		} else if marker.ModelRecognized {
			modelBacked++
		} else {
			unrecognized++
		}
	}
	return modelBacked, deterministic, unrecognized
}

func approvalProvenanceStatus(includeIssue, writeRequested, approvedLabelPresent, actorTrusted, disabledLabelPresent bool, findings []approvalProvenanceFinding) string {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "blocked"
		}
	}
	if !includeIssue {
		return "static_policy"
	}
	if disabledLabelPresent || !actorTrusted {
		return "blocked"
	}
	if writeRequested && !approvedLabelPresent {
		return "needs_approval"
	}
	for _, finding := range findings {
		if finding.Severity == "warning" {
			return "needs_review"
		}
	}
	return "ok"
}

func approvalProvenanceFindings(includeIssue, writeRequested, approvedLabelPresent, writeRequestedLabelPresent, needsHumanLabelPresent, disabledLabelPresent, actorTrusted bool, markers []approvalProvenanceMarker) []approvalProvenanceFinding {
	var findings []approvalProvenanceFinding
	add := func(severity, code, detail string) {
		findings = append(findings, approvalProvenanceFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "openclaw_exec_approval_state_separated", "approval evidence is separated from tool execution and repository mutation")
	add("info", "github_issue_label_approval_store", "GitClaw stores approval intent as current per-issue labels")
	add("info", "hermes_explicit_tool_boundary_mapped", "approval provenance is exposed as metadata without adding model-callable tools")
	add("info", "read_only_runtime_boundary", "write actions, repository mutation, and host exec remain disabled")
	if !includeIssue {
		add("info", "local_static_policy_only", "local CLI has no current issue labels or comments to prove")
		return findings
	}
	if disabledLabelPresent {
		add("error", "disabled_label_present", "the issue is disabled for GitClaw")
	}
	if !actorTrusted {
		add("error", "actor_not_trusted", "current event actor association is not trusted by configuration")
	}
	if writeRequested && !approvedLabelPresent {
		add("warning", "approval_label_missing", "write intent was detected but the approved label is absent")
	}
	if writeRequestedLabelPresent && !writeRequested {
		add("warning", "write_request_label_without_detected_write_request", "the write-requested label is present without detected write intent in the current transcript")
	}
	if approvedLabelPresent && !writeRequested {
		add("warning", "approval_label_without_write_request", "approved label is present without detected write intent")
	}
	if needsHumanLabelPresent {
		add("warning", "needs_human_label_present", "the issue is marked as needing human attention")
	}
	if len(markers) == 0 {
		add("info", "no_prior_assistant_markers", "no prior assistant-turn markers were available before this report")
	}
	for _, marker := range markers {
		if !marker.ModelRecognized {
			add("warning", "unrecognized_assistant_marker_model", "an assistant-turn marker had an unrecognized model id and was reported only by hash")
			break
		}
	}
	return findings
}

func writeApprovalLabelEvidence(b *strings.Builder, cfg Config, labels []string, includeIssue bool) {
	type labelCard struct {
		Role  string
		Label string
	}
	cards := []labelCard{
		{Role: "trigger", Label: cfg.TriggerLabel},
		{Role: "running", Label: cfg.RunningLabel},
		{Role: "done", Label: cfg.DoneLabel},
		{Role: "error", Label: cfg.ErrorLabel},
		{Role: "disabled", Label: cfg.DisabledLabel},
		{Role: "write-requested", Label: cfg.WriteRequestedLabel},
		{Role: "approved", Label: defaultApprovedLabel},
		{Role: "needs-human", Label: defaultNeedsHumanLabel},
	}
	for _, card := range cards {
		present := false
		if includeIssue {
			present = hasLabel(labels, card.Label)
		}
		fmt.Fprintf(b, "- role=`%s` label=`%s` present=`%t` label_sha256_12=`%s`\n", card.Role, card.Label, present, shortDocumentHash(card.Label))
	}
}

func writeApprovalMarkerEvidence(b *strings.Builder, markers []approvalProvenanceMarker) {
	if len(markers) == 0 {
		b.WriteString("- none\n")
		return
	}
	for i, marker := range markers {
		fmt.Fprintf(b, "- index=`%d` source=`%s` model=`%s` deterministic=`%t` has_prompt_evidence=`%t` model_recognized=`%t` model_sha256_12=`%s` context_documents=`%d` selected_skills=`%d` tool_outputs=`%d` run_id_sha256_12=`%s` event_id_sha256_12=`%s` idempotency_key_sha256_12=`%s` run_url_sha256_12=`%s` marker_sha256_12=`%s`\n",
			i+1,
			marker.Source,
			marker.Model,
			marker.Deterministic,
			marker.HasPromptEvidence,
			marker.ModelRecognized,
			marker.ModelSHA,
			marker.ContextDocuments,
			marker.SelectedSkills,
			marker.ToolOutputs,
			marker.RunIDSHA,
			marker.EventIDSHA,
			marker.IdempotencyKeySHA,
			marker.RunURLSHA,
			marker.MarkerSHA,
		)
	}
}

func writeApprovalProvenanceFindings(b *strings.Builder, findings []approvalProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}

func countPresentLabels(labels, candidates []string) int {
	count := 0
	for _, candidate := range candidates {
		if hasLabel(labels, candidate) {
			count++
		}
	}
	return count
}

func unmanagedLabelCount(labels, managed, approval []string) int {
	known := map[string]bool{}
	for _, label := range managed {
		known[label] = true
	}
	for _, label := range approval {
		known[label] = true
	}
	count := 0
	for _, label := range labels {
		if !known[label] {
			count++
		}
	}
	return count
}

func transcriptRoleCount(transcript []TranscriptMessage, role string) int {
	count := 0
	for _, message := range transcript {
		if message.Role == role {
			count++
		}
	}
	return count
}

func transcriptShape(transcript []TranscriptMessage) string {
	parts := make([]string, 0, len(transcript))
	for _, message := range transcript {
		source := "issue"
		if message.CommentID != 0 {
			source = fmt.Sprintf("comment:%d", message.CommentID)
		}
		parts = append(parts, message.Role+":"+source+":"+message.AuthorAssociation)
	}
	return strings.Join(parts, "\n")
}

func approvalMarkersEvidenceHash(markers []approvalProvenanceMarker) string {
	parts := make([]string, 0, len(markers))
	for _, marker := range markers {
		parts = append(parts, marker.Source+"|"+marker.Model+"|"+marker.MarkerSHA)
	}
	return shortDocumentHash(strings.Join(parts, "\n"))
}
