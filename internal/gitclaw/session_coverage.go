package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SessionCoverageRequirements struct {
	MinAssistantTurns      int
	MinPromptProvenance    int
	MinModelBackedTurns    int
	RequiredSkills         []string
	RequiredTools          []string
	LLME2ERequiredOnChange bool
}

type SessionCoverageReport struct {
	Scope                         string
	BackupFile                    string
	Repo                          string
	IssueNumber                   int
	EventKind                     string
	SessionCoverageStatus         string
	RequiredAssistantTurns        int
	RequiredPromptProvenanceTurns int
	RequiredModelBackedTurns      int
	RequiredSkills                []string
	RequiredTools                 []string
	RawComments                   int
	TranscriptMessages            int
	AssistantTurnComments         int
	PromptProvenanceTurns         int
	PromptContextHashMissing      int
	UniquePromptContextHashes     int
	ModelBackedTurns              int
	DeterministicTurns            int
	ModelNames                    []string
	PromptVisibleSkillNames       []string
	PromptVisibleToolNames        []string
	MissingRequiredSkills         []string
	MissingRequiredTools          []string
	AssistantTurnsMet             bool
	PromptProvenanceMet           bool
	ModelBackedTurnsMet           bool
	RequiredSkillsMet             bool
	RequiredToolsMet              bool
	RawBodiesIncluded             bool
	RawPromptsIncluded            bool
	LLME2ERequiredOnChange        bool
}

func DefaultSessionCoverageRequirements() SessionCoverageRequirements {
	return SessionCoverageRequirements{
		MinAssistantTurns:      1,
		MinPromptProvenance:    1,
		MinModelBackedTurns:    1,
		LLME2ERequiredOnChange: true,
	}
}

func (r SessionCoverageReport) OK() bool {
	return r.SessionCoverageStatus == "ok"
}

func BuildSessionCoverageReport(scope, backupFile string, ev Event, comments []Comment, transcript []TranscriptMessage, req SessionCoverageRequirements) SessionCoverageReport {
	req = normalizeSessionCoverageRequirements(req)
	provenance := buildSessionPromptProvenanceReport(comments)
	report := SessionCoverageReport{
		Scope:                         scope,
		BackupFile:                    backupFile,
		Repo:                          ev.Repo,
		IssueNumber:                   ev.Issue.Number,
		EventKind:                     ev.Kind,
		RequiredAssistantTurns:        req.MinAssistantTurns,
		RequiredPromptProvenanceTurns: req.MinPromptProvenance,
		RequiredModelBackedTurns:      req.MinModelBackedTurns,
		RequiredSkills:                normalizeSessionCoverageNames(req.RequiredSkills),
		RequiredTools:                 normalizeSessionCoverageNames(req.RequiredTools),
		RawComments:                   len(comments),
		TranscriptMessages:            len(transcript),
		PromptProvenanceTurns:         provenance.TurnsWithProvenance,
		PromptContextHashMissing:      provenance.PromptContextHashMissing,
		UniquePromptContextHashes:     provenance.UniquePromptContextSHAs,
		PromptVisibleSkillNames:       provenance.PromptVisibleSkillNames,
		PromptVisibleToolNames:        provenance.PromptVisibleToolNames,
		RawBodiesIncluded:             false,
		RawPromptsIncluded:            false,
		LLME2ERequiredOnChange:        req.LLME2ERequiredOnChange,
	}

	models := map[string]bool{}
	for _, turn := range provenance.Turns {
		report.AssistantTurnComments++
		if turn.Model == "" {
			continue
		}
		if strings.HasPrefix(turn.Model, "gitclaw/") {
			report.DeterministicTurns++
		} else {
			report.ModelBackedTurns++
		}
		if !models[turn.Model] {
			models[turn.Model] = true
			report.ModelNames = append(report.ModelNames, turn.Model)
		}
	}
	sort.Strings(report.ModelNames)

	report.MissingRequiredSkills = missingSessionCoverageNames(report.RequiredSkills, report.PromptVisibleSkillNames)
	report.MissingRequiredTools = missingSessionCoverageNames(report.RequiredTools, report.PromptVisibleToolNames)
	report.AssistantTurnsMet = report.AssistantTurnComments >= report.RequiredAssistantTurns
	report.PromptProvenanceMet = report.PromptProvenanceTurns >= report.RequiredPromptProvenanceTurns
	report.ModelBackedTurnsMet = report.ModelBackedTurns >= report.RequiredModelBackedTurns
	report.RequiredSkillsMet = len(report.MissingRequiredSkills) == 0
	report.RequiredToolsMet = len(report.MissingRequiredTools) == 0
	report.SessionCoverageStatus = "ok"
	if !report.AssistantTurnsMet || !report.PromptProvenanceMet || !report.ModelBackedTurnsMet || !report.RequiredSkillsMet || !report.RequiredToolsMet {
		report.SessionCoverageStatus = "warn"
	}
	return report
}

func RenderSessionCoverageReport(report SessionCoverageReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Coverage Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if report.Scope == "" {
		report.Scope = "issue-thread"
	}
	fmt.Fprintf(&b, "- scope: `%s`\n", report.Scope)
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- backup_file: `%s`\n", inlineCode(report.BackupFile))
	}
	if report.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", report.Repo)
	}
	if report.IssueNumber != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", report.IssueNumber)
	}
	fmt.Fprintf(&b, "- event_kind: `%s`\n", report.EventKind)
	fmt.Fprintf(&b, "- session_coverage_status: `%s`\n", report.SessionCoverageStatus)
	fmt.Fprintf(&b, "- required_assistant_turns: `%d`\n", report.RequiredAssistantTurns)
	fmt.Fprintf(&b, "- required_prompt_provenance_turns: `%d`\n", report.RequiredPromptProvenanceTurns)
	fmt.Fprintf(&b, "- required_model_backed_turns: `%d`\n", report.RequiredModelBackedTurns)
	fmt.Fprintf(&b, "- required_skill_names: `%s`\n", inlineListOrNone(report.RequiredSkills))
	fmt.Fprintf(&b, "- required_tool_names: `%s`\n", inlineListOrNone(report.RequiredTools))
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: `%d`\n", report.PromptProvenanceTurns)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: `%d`\n", report.PromptContextHashMissing)
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: `%d`\n", report.UniquePromptContextHashes)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- missing_required_skill_names: `%s`\n", inlineListOrNone(report.MissingRequiredSkills))
	fmt.Fprintf(&b, "- missing_required_tool_names: `%s`\n", inlineListOrNone(report.MissingRequiredTools))
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", report.RawPromptsIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_coverage_change: `%t`\n\n", report.LLME2ERequiredOnChange)

	b.WriteString("This report proves whether the session contains enough assistant-turn provenance to count as a real model-backed conversation. It reports counts, model names, prompt-visible skill names, prompt-visible tool names, and hashes already present in assistant markers; raw issue bodies, comment bodies, assistant replies, prompts, and tool outputs are not included.\n\n")
	b.WriteString("### Coverage Evidence\n")
	fmt.Fprintf(&b, "- assistant_turns_met=`%t`\n", report.AssistantTurnsMet)
	fmt.Fprintf(&b, "- prompt_provenance_met=`%t`\n", report.PromptProvenanceMet)
	fmt.Fprintf(&b, "- model_backed_turns_met=`%t`\n", report.ModelBackedTurnsMet)
	fmt.Fprintf(&b, "- required_skills_met=`%t`\n", report.RequiredSkillsMet)
	fmt.Fprintf(&b, "- required_tools_met=`%t`\n", report.RequiredToolsMet)
	b.WriteString("- mutation_performed=`false`\n")
	b.WriteString("- github_api_calls_performed=`false`\n")
	return strings.TrimSpace(b.String())
}

func normalizeSessionCoverageRequirements(req SessionCoverageRequirements) SessionCoverageRequirements {
	if req.MinAssistantTurns <= 0 {
		req.MinAssistantTurns = 1
	}
	if req.MinPromptProvenance <= 0 {
		req.MinPromptProvenance = 1
	}
	if req.MinModelBackedTurns <= 0 {
		req.MinModelBackedTurns = 1
	}
	req.RequiredSkills = normalizeSessionCoverageNames(req.RequiredSkills)
	req.RequiredTools = normalizeSessionCoverageNames(req.RequiredTools)
	if !req.LLME2ERequiredOnChange {
		req.LLME2ERequiredOnChange = true
	}
	return req
}

func normalizeSessionCoverageNames(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	sort.Strings(out)
	return out
}

func missingSessionCoverageNames(required, available []string) []string {
	availableSet := map[string]bool{}
	for _, value := range available {
		availableSet[value] = true
	}
	var missing []string
	for _, value := range required {
		if !availableSet[value] {
			missing = append(missing, value)
		}
	}
	return missing
}
