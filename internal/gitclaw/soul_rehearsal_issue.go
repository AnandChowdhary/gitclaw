package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const soulRehearsalIssueMarker = "gitclaw:soul-rehearsal-issue"

type SoulRehearsalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type SoulRehearsalIssueRequest struct {
	Repo               string
	Command            string
	Subcommand         string
	RehearsalID        string
	RequestedTarget    string
	TargetPath         string
	TargetCategory     string
	TargetPresent      bool
	TargetRequired     bool
	TargetCanonical    bool
	TargetLoaded       bool
	TargetSHA          string
	TargetBytes        int
	TargetLines        int
	ValidationStatus   string
	ValidationErrors   int
	ValidationWarnings int
	RiskStatus         string
	RiskFindings       int
	HighRiskFindings   int
	SourceIssueNumber  int
	SourceCommentID    int64
	SourceSHA          string
	SourceBytes        int
	SourceLines        int
	SourceKind         string
}

type SoulRehearsalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsSoulRehearsalIssueRequest(ev Event, cfg Config) bool {
	return isSoulRehearsalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isSoulRehearsalIssueFields(fields []string) bool {
	if len(fields) < 2 || fields[0] != "/soul" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse", "rehearsal", "try", "trial", "practice", "voice-test", "tone-test":
		return true
	default:
		return false
	}
}

func BuildSoulRehearsalIssueRequest(ev Event, cfg Config, repoContext RepoContext) (SoulRehearsalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isSoulRehearsalIssueFields(fields) {
		return SoulRehearsalIssueRequest{}, fmt.Errorf("missing soul rehearsal issue command")
	}
	sourceText := activeRequestText(ev)
	targetText, rehearsalID, err := parseSoulRehearsalIssueArgs(fields[2:], sourceText)
	if err != nil {
		return SoulRehearsalIssueRequest{}, err
	}
	targetPath := normalizeSoulInfoPath(targetText, cfg, repoContext)
	if targetPath == "" || !soulInfoAllowedPath(targetPath) {
		return SoulRehearsalIssueRequest{}, fmt.Errorf("unsupported soul rehearsal target %q", targetText)
	}
	if !skillNamePattern.MatchString(rehearsalID) {
		return SoulRehearsalIssueRequest{}, fmt.Errorf("invalid soul rehearsal id %q", rehearsalID)
	}
	match, ok := soulInfoMatch(cfg.Workdir, repoContext, targetPath)
	if !ok {
		return SoulRehearsalIssueRequest{}, fmt.Errorf("target metadata unavailable for %q", targetPath)
	}
	validation := ValidateSoulContext(repoContext)
	risk := BuildSoulRiskReport(repoContext)
	sourceKind := "issue"
	var sourceCommentID int64
	if ev.Comment != nil {
		sourceKind = "comment"
		sourceCommentID = ev.Comment.ID
	}
	return SoulRehearsalIssueRequest{
		Repo:               ev.Repo,
		Command:            fields[0],
		Subcommand:         strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RehearsalID:        rehearsalID,
		RequestedTarget:    cleanSoulInfoPath(targetText),
		TargetPath:         targetPath,
		TargetCategory:     match.Category,
		TargetPresent:      match.Present,
		TargetRequired:     match.Required,
		TargetCanonical:    match.Canonical,
		TargetLoaded:       match.LoadedForThisTurn,
		TargetSHA:          match.SHA,
		TargetBytes:        match.Bytes,
		TargetLines:        match.Lines,
		ValidationStatus:   validation.Status,
		ValidationErrors:   validation.Errors,
		ValidationWarnings: validation.Warnings,
		RiskStatus:         risk.Status,
		RiskFindings:       len(risk.Findings),
		HighRiskFindings:   risk.HighRiskFindings,
		SourceIssueNumber:  ev.Issue.Number,
		SourceCommentID:    sourceCommentID,
		SourceSHA:          shortDocumentHash(sourceText),
		SourceBytes:        len(sourceText),
		SourceLines:        lineCount(sourceText),
		SourceKind:         sourceKind,
	}, nil
}

func RunSoulRehearsalIssue(ctx context.Context, cfg Config, github SoulRehearsalIssueGitHubClient, req SoulRehearsalIssueRequest) (SoulRehearsalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return SoulRehearsalIssueResult{}, err
	}
	if req.RehearsalID == "" {
		return SoulRehearsalIssueResult{}, fmt.Errorf("missing soul rehearsal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return SoulRehearsalIssueResult{}, fmt.Errorf("list soul rehearsal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if soulRehearsalIssueMatches(issue.Body, req.RehearsalID) {
			return SoulRehearsalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, soulRehearsalIssueTitle(req), RenderSoulRehearsalIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return SoulRehearsalIssueResult{}, fmt.Errorf("create soul rehearsal issue: %w", err)
	}
	return SoulRehearsalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderSoulRehearsalIssueBody(req SoulRehearsalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" target_path=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", soulRehearsalIssueMarker, escapeMarkerValue(req.RehearsalID), escapeMarkerValue(req.TargetPath), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw soul rehearsal issue.\n\n")
	fmt.Fprintf(&b, "- rehearsal_id: %s\n", req.RehearsalID)
	fmt.Fprintf(&b, "- requested_target: %s\n", valueOrNone(req.RequestedTarget))
	fmt.Fprintf(&b, "- target_path: %s\n", req.TargetPath)
	fmt.Fprintf(&b, "- target_category: %s\n", req.TargetCategory)
	fmt.Fprintf(&b, "- target_present: %t\n", req.TargetPresent)
	fmt.Fprintf(&b, "- target_required: %t\n", req.TargetRequired)
	fmt.Fprintf(&b, "- target_canonical: %t\n", req.TargetCanonical)
	fmt.Fprintf(&b, "- target_loaded_for_this_turn: %t\n", req.TargetLoaded)
	fmt.Fprintf(&b, "- target_sha256_12: %s\n", valueOrNone(req.TargetSHA))
	fmt.Fprintf(&b, "- target_bytes: %d\n", req.TargetBytes)
	fmt.Fprintf(&b, "- target_lines: %d\n", req.TargetLines)
	fmt.Fprintf(&b, "- soul_validation_status: %s\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- soul_risk_status: %s\n", req.RiskStatus)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: %d\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: %d\n", req.SourceLines)
	b.WriteString("- rehearsal_mode: github-issue-conversation\n")
	b.WriteString("- context_target_write_allowed: false\n")
	b.WriteString("- candidate_soul_generation_allowed: false\n")
	b.WriteString("- repository_mutation_allowed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_target_body_included: false\n")
	b.WriteString("- raw_candidate_soul_included: false\n\n")
	fmt.Fprintf(&b, "Use this issue to rehearse the current `%s` behavior in a normal GitClaw conversation. Proposed changes belong in a reviewed `/soul propose` issue or pull request; this issue is only for trying the current prompt-visible behavior.\n", req.TargetPath)
	return strings.TrimSpace(b.String())
}

func RenderSoulRehearsalIssueActionReport(ev Event, req SoulRehearsalIssueRequest, result SoulRehearsalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Soul Rehearsal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_soul_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- soul_rehearsal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rehearsal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- rehearsal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- rehearsal_id_sha256_12: `%s`\n", shortDocumentHash(req.RehearsalID))
	fmt.Fprintf(&b, "- requested_target: `%s`\n", inlineCode(req.RequestedTarget))
	fmt.Fprintf(&b, "- normalized_soul_path: `%s`\n", req.TargetPath)
	fmt.Fprintf(&b, "- target_category: `%s`\n", req.TargetCategory)
	fmt.Fprintf(&b, "- target_present: `%t`\n", req.TargetPresent)
	fmt.Fprintf(&b, "- target_required: `%t`\n", req.TargetRequired)
	fmt.Fprintf(&b, "- target_canonical: `%t`\n", req.TargetCanonical)
	fmt.Fprintf(&b, "- target_loaded_for_this_turn: `%t`\n", req.TargetLoaded)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", valueOrNone(req.TargetSHA))
	fmt.Fprintf(&b, "- target_bytes: `%d`\n", req.TargetBytes)
	fmt.Fprintf(&b, "- target_lines: `%d`\n", req.TargetLines)
	fmt.Fprintf(&b, "- soul_validation_status: `%s`\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- soul_validation_errors: `%d`\n", req.ValidationErrors)
	fmt.Fprintf(&b, "- soul_validation_warnings: `%d`\n", req.ValidationWarnings)
	fmt.Fprintf(&b, "- soul_risk_status: `%s`\n", req.RiskStatus)
	fmt.Fprintf(&b, "- soul_risk_findings: `%d`\n", req.RiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", req.HighRiskFindings)
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- context_target_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- candidate_soul_generation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_candidate_soul_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_soul_rehearsal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for rehearsing the current high-authority context in a normal conversation. The action does not generate candidate soul text, write `.gitclaw/` files, mutate the repository, or call a model; continue on the rehearsal issue to exercise the current behavior with GitHub Models.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- ask a normal `@gitclaw` follow-up that relies on `%s` without requesting a file edit\n", req.TargetPath)
	b.WriteString("- verify the follow-up assistant marker includes prompt context, selected skill, prompt-visible tools, and usage telemetry\n")
	return strings.TrimSpace(b.String())
}

func parseSoulRehearsalIssueArgs(args []string, sourceText string) (string, string, error) {
	target := "soul"
	targetSet := false
	rehearsalID := ""
	for i := 0; i < len(args); i++ {
		field := strings.TrimSpace(args[i])
		if field == "" {
			continue
		}
		switch field {
		case "--target", "--path":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("%s requires a value", field)
			}
			target = cleanSoulInfoPath(args[i])
			targetSet = true
		case "--id":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--id requires a value")
			}
			rehearsalID = cleanSoulRehearsalID(args[i])
		default:
			if strings.HasPrefix(field, "--") {
				return "", "", fmt.Errorf("unknown soul rehearsal argument %q", field)
			}
			if !targetSet {
				target = cleanSoulInfoPath(field)
				targetSet = true
			}
		}
	}
	if rehearsalID == "" {
		rehearsalID = cleanSoulRehearsalID("soul-rehearsal-" + shortDocumentHash(sourceText))
	}
	return target, rehearsalID, nil
}

func cleanSoulRehearsalID(value string) string {
	return cleanSkillRehearsalID(value)
}

func soulRehearsalIssueTitle(req SoulRehearsalIssueRequest) string {
	title := "GitClaw soul rehearsal: " + req.TargetPath
	if req.RehearsalID != "" {
		title += " (" + req.RehearsalID + ")"
	}
	return title
}

func soulRehearsalIssueMatches(body, rehearsalID string) bool {
	return strings.Contains(body, "<!-- "+soulRehearsalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(cleanSoulRehearsalID(rehearsalID))))
}
