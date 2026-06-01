package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const soulProposalIssueMarker = "gitclaw:soul-proposal-issue"

type SoulProposalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type SoulProposalIssueRequest struct {
	Repo               string
	Command            string
	Subcommand         string
	ProposalID         string
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

type SoulProposalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsSoulProposalIssueRequest(ev Event, cfg Config) bool {
	return isSoulProposalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isSoulProposalIssueFields(fields []string) bool {
	if len(fields) < 2 || fields[0] != "/soul" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose", "proposal", "proposal-issue", "edit", "change":
		return true
	default:
		return false
	}
}

func BuildSoulProposalIssueRequest(ev Event, cfg Config, repoContext RepoContext) (SoulProposalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isSoulProposalIssueFields(fields) {
		return SoulProposalIssueRequest{}, fmt.Errorf("missing soul proposal issue command")
	}
	sourceText := activeRequestText(ev)
	targetText, proposalID, err := parseSoulProposalIssueArgs(fields[2:], sourceText)
	if err != nil {
		return SoulProposalIssueRequest{}, err
	}
	targetPath := normalizeSoulInfoPath(targetText, cfg, repoContext)
	if targetPath == "" || !soulInfoAllowedPath(targetPath) {
		return SoulProposalIssueRequest{}, fmt.Errorf("unsupported soul proposal target %q", targetText)
	}
	if !skillNamePattern.MatchString(proposalID) {
		return SoulProposalIssueRequest{}, fmt.Errorf("invalid soul proposal id %q", proposalID)
	}
	match, ok := soulInfoMatch(cfg.Workdir, repoContext, targetPath)
	if !ok {
		return SoulProposalIssueRequest{}, fmt.Errorf("target metadata unavailable for %q", targetPath)
	}
	validation := ValidateSoulContext(repoContext)
	risk := BuildSoulRiskReport(repoContext)
	sourceKind := "issue"
	var sourceCommentID int64
	if ev.Comment != nil {
		sourceKind = "comment"
		sourceCommentID = ev.Comment.ID
	}
	return SoulProposalIssueRequest{
		Repo:               ev.Repo,
		Command:            fields[0],
		Subcommand:         strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		ProposalID:         proposalID,
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

func RunSoulProposalIssue(ctx context.Context, github SoulProposalIssueGitHubClient, req SoulProposalIssueRequest) (SoulProposalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return SoulProposalIssueResult{}, err
	}
	if req.ProposalID == "" {
		return SoulProposalIssueResult{}, fmt.Errorf("missing soul proposal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, nil, 300)
	if err != nil {
		return SoulProposalIssueResult{}, fmt.Errorf("list soul proposal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if soulProposalIssueMatches(issue.Body, req.ProposalID) {
			return SoulProposalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	title := fmt.Sprintf("GitClaw soul proposal: %s", req.ProposalID)
	issue, err := github.CreateIssue(ctx, req.Repo, title, RenderSoulProposalIssueBody(req), nil)
	if err != nil {
		return SoulProposalIssueResult{}, fmt.Errorf("create soul proposal issue: %w", err)
	}
	return SoulProposalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderSoulProposalIssueBody(req SoulProposalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" target_path=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", soulProposalIssueMarker, escapeMarkerValue(req.ProposalID), escapeMarkerValue(req.TargetPath), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw soul proposal issue.\n\n")
	fmt.Fprintf(&b, "- proposal_id: %s\n", req.ProposalID)
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
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: %d\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: %d\n", req.SourceLines)
	b.WriteString("- review_pr_required: true\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_candidate_soul_included: false\n")
	b.WriteString("- raw_existing_soul_included: false\n")
	b.WriteString("- soul_file_written: false\n\n")
	fmt.Fprintf(&b, "Review this issue, then draft a high-authority context change for `%s` on a normal code-review branch if the proposal is worth keeping. GitClaw does not write soul/profile files from this issue.", req.TargetPath)
	return b.String()
}

func RenderSoulProposalIssueActionReport(ev Event, req SoulProposalIssueRequest, result SoulProposalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Soul Proposal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_soul_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- soul_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- soul_proposal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- soul_proposal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- soul_proposal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- soul_proposal_id: `%s`\n", inlineCode(req.ProposalID))
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
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-soul-file")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_candidate_soul_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_existing_soul_included: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_soul_proposal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for a high-authority context proposal. The issue is a review queue entry, not an applied profile change: `.gitclaw/SOUL.md`, `.gitclaw/IDENTITY.md`, `.gitclaw/USER.md`, and related context files are not written, candidate content is not generated, and raw source request text is not copied into this receipt.\n\n")
	b.WriteString("### Review Path\n")
	fmt.Fprintf(&b, "- review proposal issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- if accepted, draft a high-authority context diff for `%s` on a normal branch\n", req.TargetPath)
	b.WriteString("- run `gitclaw soul validate`, `gitclaw soul verify`, `gitclaw profile verify`, and a live GitHub Models conversation E2E before promotion\n")
	return strings.TrimSpace(b.String())
}

func parseSoulProposalIssueArgs(args []string, sourceText string) (string, string, error) {
	target := "soul"
	targetSet := false
	proposalID := ""
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
			proposalID = cleanSoulProposalID(args[i])
		default:
			if strings.HasPrefix(field, "--") {
				return "", "", fmt.Errorf("unknown soul proposal argument %q", field)
			}
			if !targetSet {
				target = cleanSoulInfoPath(field)
				targetSet = true
			}
		}
	}
	if proposalID == "" {
		proposalID = "soul-" + shortDocumentHash(sourceText)
	}
	return target, proposalID, nil
}

func cleanSoulProposalID(id string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(id)), " \t\r\n.,:;!?`\"'")
}

func soulProposalIssueMatches(body, proposalID string) bool {
	return strings.Contains(body, "<!-- "+soulProposalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(proposalID)))
}
