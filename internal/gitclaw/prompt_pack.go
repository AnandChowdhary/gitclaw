package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

const (
	promptPackAgentCompressionPct  = 50
	promptPackGatewayHygienePct    = 85
	promptPackEstimatedCharsPerTok = 4
)

type PromptPackComponent struct {
	Kind          string
	Name          string
	Index         int
	PackStatus    string
	PackReason    string
	Bytes         int
	Lines         int
	SHA           string
	StartByte     int
	EndByte       int
	BodyIncluded  bool
	InputIncluded bool
	Metadata      []string
}

type PromptPackReport struct {
	Status                              string
	PackStrategy                        string
	CompressionModel                    string
	Provider                            string
	Model                               string
	MaxPromptBytes                      int
	MaxOutputTokens                     int
	SystemPromptBytes                   int
	SystemPromptSHA                     string
	FullUserPromptBytes                 int
	PackedUserPromptBytes               int
	UserPromptSHA                       string
	TotalModelInputBytes                int
	EstimatedInputTokens                int
	UserPromptBudgetPercent             int
	AgentCompressionThresholdPercent    int
	AgentCompressionThresholdBytes      int
	GatewayHygieneThresholdPercent      int
	GatewayHygieneThresholdBytes        int
	PromptContainsTruncation            bool
	PackTruncationMarkerBytes           int
	Components                          int
	PackedComponents                    int
	PartialComponents                   int
	OmittedComponents                   int
	SeparateSystemComponents            int
	ContextFiles                        int
	SelectedSkills                      int
	ToolOutputs                         int
	TranscriptMessages                  int
	BoundedTranscriptMessages           int
	OmittedOlderMessages                int
	TruncatedTranscriptBodies           int
	PromptBodyIncluded                  bool
	ContextFileBodiesIncluded           bool
	SkillBodiesIncluded                 bool
	ToolOutputBodiesIncluded            bool
	RawToolInputsIncluded               bool
	RawIssueBodiesIncluded              bool
	RawCommentBodiesIncluded            bool
	CredentialValuesIncluded            bool
	RepositoryMutationAllowed           bool
	LLME2ERequiredAfterPromptPackChange bool
	Findings                            []PromptRiskFinding
	HighRiskFindings                    int
	WarningRiskFindings                 int
	InfoRiskFindings                    int
	Cards                               []PromptPackComponent
}

func IsPromptPackRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return isPromptPackFields(fields)
}

func isPromptPackFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/prompt" && fields[0] != "/budget" && fields[0] != "/prompt-budget" {
		return false
	}
	return strings.EqualFold(fields[1], "pack") ||
		strings.EqualFold(fields[1], "pack-plan") ||
		strings.EqualFold(fields[1], "packing") ||
		strings.EqualFold(fields[1], "context-pack")
}

func RenderPromptPackReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) string {
	return renderPromptPackReport(ev, cfg, transcript, repoContext, true)
}

func RenderPromptPackCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPromptPackReport(Event{}, cfg, nil, repoContext, false)
}

func renderPromptPackReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext, includeIssue bool) string {
	report := BuildPromptPackReport(ev, cfg, transcript, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Prompt Pack Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writePromptPackSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report explains GitClaw's deterministic prompt packing order and final prompt-budget projection. It reports component paths, names, sizes, hashes, statuses, and threshold findings only; prompt text, issue bodies, comments, context bodies, skill bodies, tool outputs, raw tool inputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Pack Components\n")
	writePromptPackCards(&b, report.Cards)

	b.WriteString("\n### Findings\n")
	writePromptRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildPromptPackReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) PromptPackReport {
	budget := promptBudgetConfig(cfg)
	components, fullPrompt := promptPackComponents(ev, budget, transcript, repoContext)
	components = clampPromptPackComponentRanges(components, len(fullPrompt))
	packedPrompt := truncatePromptText(fullPrompt, budget.MaxPromptBytes)
	report := PromptPackReport{
		Status:                              "ok",
		PackStrategy:                        "fixed-order-head-tail-budgeted",
		CompressionModel:                    "openclaw-budget-snapshot+hermes-50-85-thresholds",
		Provider:                            cfg.ModelProvider,
		Model:                               cfg.Model,
		MaxPromptBytes:                      budget.MaxPromptBytes,
		MaxOutputTokens:                     cfg.MaxOutputTokens,
		SystemPromptBytes:                   len(systemPrompt),
		SystemPromptSHA:                     shortDocumentHash(systemPrompt),
		FullUserPromptBytes:                 len(fullPrompt),
		PackedUserPromptBytes:               len(packedPrompt),
		UserPromptSHA:                       shortDocumentHash(packedPrompt),
		TotalModelInputBytes:                len(systemPrompt) + len(packedPrompt),
		EstimatedInputTokens:                estimatePromptTokens(len(systemPrompt) + len(packedPrompt)),
		UserPromptBudgetPercent:             percentOf(len(packedPrompt), budget.MaxPromptBytes),
		AgentCompressionThresholdPercent:    promptPackAgentCompressionPct,
		AgentCompressionThresholdBytes:      budget.MaxPromptBytes * promptPackAgentCompressionPct / 100,
		GatewayHygieneThresholdPercent:      promptPackGatewayHygienePct,
		GatewayHygieneThresholdBytes:        budget.MaxPromptBytes * promptPackGatewayHygienePct / 100,
		PromptContainsTruncation:            strings.Contains(packedPrompt, "[gitclaw:truncated"),
		PackTruncationMarkerBytes:           promptTruncationMarkerBytes(len(fullPrompt), budget.MaxPromptBytes),
		Components:                          len(components),
		ContextFiles:                        len(repoContext.Documents),
		SelectedSkills:                      len(repoContext.Skills),
		ToolOutputs:                         len(repoContext.ToolOutputs),
		TranscriptMessages:                  len(transcript),
		PromptBodyIncluded:                  false,
		ContextFileBodiesIncluded:           false,
		SkillBodiesIncluded:                 false,
		ToolOutputBodiesIncluded:            false,
		RawToolInputsIncluded:               false,
		RawIssueBodiesIncluded:              false,
		RawCommentBodiesIncluded:            false,
		CredentialValuesIncluded:            false,
		RepositoryMutationAllowed:           false,
		LLME2ERequiredAfterPromptPackChange: true,
		Cards:                               applyPromptPackProjection(components, len(fullPrompt), budget.MaxPromptBytes),
	}
	bounded, omitted := boundedTranscript(transcript, budget.MaxTranscriptMessages)
	report.BoundedTranscriptMessages = len(bounded)
	report.OmittedOlderMessages = omitted
	for _, msg := range bounded {
		if len(msg.Body) > budget.MaxTranscriptMessageBytes {
			report.TruncatedTranscriptBodies++
		}
	}
	for _, card := range report.Cards {
		switch card.PackStatus {
		case "packed":
			report.PackedComponents++
		case "partial":
			report.PartialComponents++
		case "omitted":
			report.OmittedComponents++
		case "separate":
			report.SeparateSystemComponents++
		}
	}
	report.Findings = promptPackFindings(report)
	sortPromptRiskFindings(report.Findings)
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high":
			report.HighRiskFindings++
		case "warning":
			report.WarningRiskFindings++
		default:
			report.InfoRiskFindings++
		}
	}
	switch {
	case report.HighRiskFindings > 0:
		report.Status = "high"
	case report.WarningRiskFindings > 0:
		report.Status = "warn"
	default:
		report.Status = "ok"
	}
	return report
}

func promptPackComponents(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) ([]PromptPackComponent, string) {
	var b strings.Builder
	var cards []PromptPackComponent
	add := func(kind, name, text string, bodyIncluded, inputIncluded bool, metadata []string) {
		start := b.Len()
		b.WriteString(text)
		end := b.Len()
		cards = append(cards, PromptPackComponent{
			Kind:          kind,
			Name:          name,
			Index:         len(cards) + 1,
			Bytes:         len(text),
			Lines:         lineCount(text),
			SHA:           shortDocumentHash(text),
			StartByte:     start,
			EndByte:       end,
			BodyIncluded:  bodyIncluded,
			InputIncluded: inputIncluded,
			Metadata:      uniqueSortedStrings(metadata),
		})
	}

	header := fmt.Sprintf("Repository: %s\nIssue: #%d %s\n\n", ev.Repo, ev.Issue.Number, ev.Issue.Title)
	add("run-header", "repository-and-issue", header, false, false, nil)
	if len(repoContext.Documents) > 0 || len(repoContext.Skills) > 0 || len(repoContext.ToolOutputs) > 0 {
		add("section-header", "repository-context", "Repository context:\n", false, false, nil)
		for _, doc := range repoContext.Documents {
			text := fmt.Sprintf("\n[context_file path=%s]\n%s\n", doc.Path, doc.Body)
			add("context-file", doc.Path, text, false, false, []string{fmt.Sprintf("source_bytes:%d", len(doc.Body))})
		}
		for _, skill := range repoContext.Skills {
			text := fmt.Sprintf("\n[skill path=%s]\n%s\n", skill.Path, skill.Body)
			add("selected-skill", skill.Path, text, false, false, []string{fmt.Sprintf("source_bytes:%d", len(skill.Body))})
		}
		for _, output := range repoContext.ToolOutputs {
			text := fmt.Sprintf("\n[tool_output name=%s input=%s]\n%s\n", output.Name, output.Input, output.Output)
			add("tool-output", output.Name, text, false, false, []string{
				"input_sha256_12:" + shortDocumentHash(output.Input),
				fmt.Sprintf("output_bytes:%d", len(output.Output)),
				"output_sha256_12:" + shortDocumentHash(output.Output),
			})
		}
		add("section-separator", "repository-context-end", "\n", false, false, nil)
	}
	add("section-header", "transcript", "Transcript:\n", false, false, nil)
	bounded, omitted := boundedTranscript(transcript, cfg.MaxTranscriptMessages)
	if omitted > 0 {
		add("transcript-omission-marker", "omitted-older-messages", fmt.Sprintf("\n[gitclaw.prompt_budget omitted_older_messages=%d]\n", omitted), false, false, []string{fmt.Sprintf("omitted:%d", omitted)})
	}
	for index, msg := range bounded {
		trust := "untrusted"
		if msg.Trusted {
			trust = "trusted"
		}
		body := truncatePromptText(msg.Body, cfg.MaxTranscriptMessageBytes)
		text := fmt.Sprintf("\n[%s %s actor=%s association=%s comment_id=%d edited=%v]\n%s\n", msg.Role, trust, msg.Actor, msg.AuthorAssociation, msg.CommentID, msg.Edited, body)
		name := fmt.Sprintf("%s-%02d", msg.Role, index+1)
		add("transcript-message", name, text, false, false, []string{
			"trusted:" + fmt.Sprintf("%t", msg.Trusted),
			"edited:" + fmt.Sprintf("%t", msg.Edited),
			"body_sha256_12:" + shortDocumentHash(body),
		})
	}
	return cards, strings.TrimSpace(b.String())
}

func clampPromptPackComponentRanges(cards []PromptPackComponent, fullBytes int) []PromptPackComponent {
	out := make([]PromptPackComponent, len(cards))
	copy(out, cards)
	for i := range out {
		if out[i].StartByte > fullBytes {
			out[i].StartByte = fullBytes
		}
		if out[i].EndByte > fullBytes {
			out[i].EndByte = fullBytes
		}
		if out[i].EndByte < out[i].StartByte {
			out[i].EndByte = out[i].StartByte
		}
	}
	return out
}

func applyPromptPackProjection(cards []PromptPackComponent, fullBytes, maxBytes int) []PromptPackComponent {
	out := make([]PromptPackComponent, len(cards))
	copy(out, cards)
	if maxBytes <= 0 || fullBytes <= maxBytes {
		for i := range out {
			out[i].PackStatus = "packed"
			out[i].PackReason = "within_budget"
		}
		return out
	}
	markerBytes := promptTruncationMarkerBytes(fullBytes, maxBytes)
	keep := maxBytes - markerBytes
	if keep <= 0 {
		for i := range out {
			out[i].PackStatus = "omitted"
			out[i].PackReason = "budget_smaller_than_marker"
		}
		return out
	}
	headEnd := keep / 2
	tailStart := fullBytes - (keep - headEnd)
	for i := range out {
		card := &out[i]
		headOverlap := rangesOverlap(card.StartByte, card.EndByte, 0, headEnd)
		tailOverlap := rangesOverlap(card.StartByte, card.EndByte, tailStart, fullBytes)
		switch {
		case (card.StartByte >= 0 && card.EndByte <= headEnd) || card.StartByte >= tailStart:
			card.PackStatus = "packed"
			if card.EndByte <= headEnd {
				card.PackReason = "head_preserved"
			} else {
				card.PackReason = "tail_preserved"
			}
		case headOverlap || tailOverlap:
			card.PackStatus = "partial"
			card.PackReason = "crosses_truncation_boundary"
		default:
			card.PackStatus = "omitted"
			card.PackReason = "middle_elided"
		}
	}
	return out
}

func rangesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	return aStart < bEnd && bStart < aEnd
}

func promptTruncationMarkerBytes(fullBytes, maxBytes int) int {
	if maxBytes <= 0 || fullBytes <= maxBytes {
		return 0
	}
	marker := fmt.Sprintf("\n...[gitclaw:truncated omitted_bytes=%d]...\n", fullBytes-maxBytes)
	if maxBytes <= len(marker)+20 {
		return maxBytes
	}
	return len(marker)
}

func estimatePromptTokens(bytes int) int {
	if bytes <= 0 {
		return 0
	}
	return (bytes + promptPackEstimatedCharsPerTok - 1) / promptPackEstimatedCharsPerTok
}

func promptPackFindings(report PromptPackReport) []PromptRiskFinding {
	var findings []PromptRiskFinding
	add := func(severity, code, category, kind, path, field, value string) {
		findings = append(findings, PromptRiskFinding{
			Severity: severity,
			Code:     code,
			Category: category,
			Kind:     kind,
			Path:     path,
			Field:    field,
			LineSHA:  shortDocumentHash(value),
		})
	}
	add("info", "prompt_pack_order_static", "prompt-pack", "prompt-pack", "gitclaw", "pack_strategy", report.PackStrategy)
	add("info", "openclaw_context_budget_snapshot", "prompt-budget", "prompt-pack", "gitclaw", "max_prompt_bytes", fmt.Sprintf("%d", report.MaxPromptBytes))
	add("info", "hermes_compression_thresholds_evaluated", "context-compression", "prompt-pack", "gitclaw", "compression_model", report.CompressionModel)
	if report.PromptContainsTruncation || report.FullUserPromptBytes > report.MaxPromptBytes {
		add("warning", "prompt_pack_requires_final_truncation", "prompt-budget", "prompt-pack", "gitclaw", "full_user_prompt_bytes", fmt.Sprintf("%d", report.FullUserPromptBytes))
	}
	if report.FullUserPromptBytes >= report.AgentCompressionThresholdBytes && report.AgentCompressionThresholdBytes > 0 {
		add("warning", "prompt_over_agent_compression_threshold", "context-compression", "prompt-pack", "gitclaw", "agent_threshold", fmt.Sprintf("%d", report.AgentCompressionThresholdBytes))
	}
	if report.FullUserPromptBytes >= report.GatewayHygieneThresholdBytes && report.GatewayHygieneThresholdBytes > 0 {
		add("warning", "prompt_over_gateway_hygiene_threshold", "context-compression", "prompt-pack", "gitclaw", "gateway_threshold", fmt.Sprintf("%d", report.GatewayHygieneThresholdBytes))
	}
	if report.OmittedOlderMessages > 0 {
		add("info", "older_transcript_messages_omitted", "transcript-budget", "transcript", "transcript", "omitted_older_messages", fmt.Sprintf("%d", report.OmittedOlderMessages))
	}
	if report.TruncatedTranscriptBodies > 0 {
		add("warning", "transcript_message_bodies_truncated", "transcript-budget", "transcript", "transcript", "truncated_transcript_bodies", fmt.Sprintf("%d", report.TruncatedTranscriptBodies))
	}
	sortPromptRiskFindings(findings)
	return findings
}

func writePromptPackSummary(b *strings.Builder, report PromptPackReport) {
	fmt.Fprintf(b, "- prompt_pack_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- pack_strategy: `%s`\n", report.PackStrategy)
	fmt.Fprintf(b, "- compression_model: `%s`\n", report.CompressionModel)
	fmt.Fprintf(b, "- provider: `%s`\n", report.Provider)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- max_prompt_bytes: `%d`\n", report.MaxPromptBytes)
	fmt.Fprintf(b, "- max_output_tokens: `%d`\n", report.MaxOutputTokens)
	fmt.Fprintf(b, "- system_prompt_bytes: `%d`\n", report.SystemPromptBytes)
	fmt.Fprintf(b, "- system_prompt_sha256_12: `%s`\n", report.SystemPromptSHA)
	fmt.Fprintf(b, "- full_user_prompt_bytes: `%d`\n", report.FullUserPromptBytes)
	fmt.Fprintf(b, "- packed_user_prompt_bytes: `%d`\n", report.PackedUserPromptBytes)
	fmt.Fprintf(b, "- user_prompt_sha256_12: `%s`\n", report.UserPromptSHA)
	fmt.Fprintf(b, "- total_model_input_bytes: `%d`\n", report.TotalModelInputBytes)
	fmt.Fprintf(b, "- estimated_input_tokens: `%d`\n", report.EstimatedInputTokens)
	fmt.Fprintf(b, "- user_prompt_budget_percent: `%d`\n", report.UserPromptBudgetPercent)
	fmt.Fprintf(b, "- agent_compression_threshold_percent: `%d`\n", report.AgentCompressionThresholdPercent)
	fmt.Fprintf(b, "- agent_compression_threshold_bytes: `%d`\n", report.AgentCompressionThresholdBytes)
	fmt.Fprintf(b, "- gateway_hygiene_threshold_percent: `%d`\n", report.GatewayHygieneThresholdPercent)
	fmt.Fprintf(b, "- gateway_hygiene_threshold_bytes: `%d`\n", report.GatewayHygieneThresholdBytes)
	fmt.Fprintf(b, "- prompt_contains_truncation_marker: `%t`\n", report.PromptContainsTruncation)
	fmt.Fprintf(b, "- pack_truncation_marker_bytes: `%d`\n", report.PackTruncationMarkerBytes)
	fmt.Fprintf(b, "- pack_components: `%d`\n", report.Components)
	fmt.Fprintf(b, "- packed_components: `%d`\n", report.PackedComponents)
	fmt.Fprintf(b, "- partial_components: `%d`\n", report.PartialComponents)
	fmt.Fprintf(b, "- omitted_components: `%d`\n", report.OmittedComponents)
	fmt.Fprintf(b, "- separate_system_components: `%d`\n", report.SeparateSystemComponents)
	fmt.Fprintf(b, "- context_files: `%d`\n", report.ContextFiles)
	fmt.Fprintf(b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(b, "- tool_outputs: `%d`\n", report.ToolOutputs)
	fmt.Fprintf(b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(b, "- bounded_transcript_messages: `%d`\n", report.BoundedTranscriptMessages)
	fmt.Fprintf(b, "- omitted_older_messages: `%d`\n", report.OmittedOlderMessages)
	fmt.Fprintf(b, "- truncated_transcript_bodies: `%d`\n", report.TruncatedTranscriptBodies)
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- prompt_body_included: `%t`\n", report.PromptBodyIncluded)
	fmt.Fprintf(b, "- context_file_bodies_included: `%t`\n", report.ContextFileBodiesIncluded)
	fmt.Fprintf(b, "- skill_bodies_included: `%t`\n", report.SkillBodiesIncluded)
	fmt.Fprintf(b, "- tool_output_bodies_included: `%t`\n", report.ToolOutputBodiesIncluded)
	fmt.Fprintf(b, "- raw_tool_inputs_included: `%t`\n", report.RawToolInputsIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- llm_e2e_required_after_prompt_pack_change: `%t`\n", report.LLME2ERequiredAfterPromptPackChange)
}

func writePromptPackCards(b *strings.Builder, cards []PromptPackComponent) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	sort.SliceStable(cards, func(i, j int) bool { return cards[i].Index < cards[j].Index })
	for _, card := range cards {
		fmt.Fprintf(
			b,
			"- index=`%d` kind=`%s` name=`%s` pack_status=`%s` reason=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` start_byte=`%d` end_byte=`%d` body_included=`%t` input_included=`%t` metadata=`%s`\n",
			card.Index,
			card.Kind,
			inlineCode(card.Name),
			card.PackStatus,
			card.PackReason,
			card.Bytes,
			card.Lines,
			card.SHA,
			card.StartByte,
			card.EndByte,
			card.BodyIncluded,
			card.InputIncluded,
			inlineListOrNone(card.Metadata),
		)
	}
}
