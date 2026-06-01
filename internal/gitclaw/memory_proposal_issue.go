package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const memoryProposalIssueMarker = "gitclaw:memory-proposal-issue"

type MemoryProposalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type MemoryProposalIssueRequest struct {
	Repo               string
	Command            string
	Subcommand         string
	ProposalID         string
	Target             memoryPromoteTarget
	TargetPresent      bool
	TargetSHA          string
	TargetBytes        int
	TargetLines        int
	DatedMemoryNotes   int
	LatestMemoryNote   string
	MemoryBudget       int
	RemainingBytes     int
	ValidationStatus   string
	ValidationErrors   int
	ValidationWarnings int
	SourceIssueNumber  int
	SourceCommentID    int64
	SourceSHA          string
	SourceBytes        int
	SourceLines        int
	SourceKind         string
}

type MemoryProposalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsMemoryProposalIssueRequest(ev Event, cfg Config) bool {
	return isMemoryProposalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isMemoryProposalIssueFields(fields []string) bool {
	if len(fields) < 2 || (fields[0] != "/memory" && fields[0] != "/memories") {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "remember", "remember-issue", "memory-proposal", "proposal", "propose":
		return true
	default:
		return false
	}
}

func BuildMemoryProposalIssueRequest(ev Event, cfg Config, repoContext RepoContext) (MemoryProposalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isMemoryProposalIssueFields(fields) {
		return MemoryProposalIssueRequest{}, fmt.Errorf("missing memory proposal issue command")
	}
	sourceText := activeRequestText(ev)
	targetText, proposalID, err := parseMemoryProposalIssueArgs(fields[2:], sourceText)
	if err != nil {
		return MemoryProposalIssueRequest{}, err
	}
	target := normalizeMemoryPromoteTarget(targetText)
	if !target.Supported {
		return MemoryProposalIssueRequest{}, fmt.Errorf("unsupported memory proposal target %q", target.Requested)
	}
	if !skillNamePattern.MatchString(proposalID) {
		return MemoryProposalIssueRequest{}, fmt.Errorf("invalid memory proposal id %q", proposalID)
	}
	surface := inspectMemorySurface(cfg.Workdir, repoContext)
	validation := ValidateMemory(cfg.Workdir, repoContext)
	targetFile := memoryPromoteTargetFile(surface, target)
	remainingBytes := maxContextDocumentBytes - targetFile.Bytes
	if remainingBytes < 0 {
		remainingBytes = 0
	}
	sourceKind := "issue"
	var sourceCommentID int64
	if ev.Comment != nil {
		sourceKind = "comment"
		sourceCommentID = ev.Comment.ID
	}
	return MemoryProposalIssueRequest{
		Repo:               ev.Repo,
		Command:            fields[0],
		Subcommand:         strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		ProposalID:         proposalID,
		Target:             target,
		TargetPresent:      targetFile.Present,
		TargetSHA:          targetFile.SHA,
		TargetBytes:        targetFile.Bytes,
		TargetLines:        targetFile.Lines,
		DatedMemoryNotes:   len(surface.DatedNotes),
		LatestMemoryNote:   latestMemoryNotePath(surface.DatedNotes),
		MemoryBudget:       maxContextDocumentBytes,
		RemainingBytes:     remainingBytes,
		ValidationStatus:   validation.Status,
		ValidationErrors:   validation.Errors,
		ValidationWarnings: validation.Warnings,
		SourceIssueNumber:  ev.Issue.Number,
		SourceCommentID:    sourceCommentID,
		SourceSHA:          shortDocumentHash(sourceText),
		SourceBytes:        len(sourceText),
		SourceLines:        lineCount(sourceText),
		SourceKind:         sourceKind,
	}, nil
}

func RunMemoryProposalIssue(ctx context.Context, github MemoryProposalIssueGitHubClient, req MemoryProposalIssueRequest) (MemoryProposalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return MemoryProposalIssueResult{}, err
	}
	if req.ProposalID == "" {
		return MemoryProposalIssueResult{}, fmt.Errorf("missing memory proposal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, nil, 300)
	if err != nil {
		return MemoryProposalIssueResult{}, fmt.Errorf("list memory proposal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if memoryProposalIssueMatches(issue.Body, req.ProposalID) {
			return MemoryProposalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	title := fmt.Sprintf("GitClaw memory proposal: %s", req.ProposalID)
	issue, err := github.CreateIssue(ctx, req.Repo, title, RenderMemoryProposalIssueBody(req), nil)
	if err != nil {
		return MemoryProposalIssueResult{}, fmt.Errorf("create memory proposal issue: %w", err)
	}
	return MemoryProposalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderMemoryProposalIssueBody(req MemoryProposalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" target_kind=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", memoryProposalIssueMarker, escapeMarkerValue(req.ProposalID), escapeMarkerValue(req.Target.Kind), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw memory proposal issue.\n\n")
	fmt.Fprintf(&b, "- proposal_id: %s\n", req.ProposalID)
	fmt.Fprintf(&b, "- target_kind: %s\n", req.Target.Kind)
	fmt.Fprintf(&b, "- target_path: %s\n", req.Target.Path)
	fmt.Fprintf(&b, "- target_present: %t\n", req.TargetPresent)
	fmt.Fprintf(&b, "- target_sha256_12: %s\n", valueOrNone(req.TargetSHA))
	fmt.Fprintf(&b, "- target_bytes: %d\n", req.TargetBytes)
	fmt.Fprintf(&b, "- memory_budget_bytes: %d\n", req.MemoryBudget)
	fmt.Fprintf(&b, "- memory_budget_remaining_bytes: %d\n", req.RemainingBytes)
	fmt.Fprintf(&b, "- dated_memory_notes: %d\n", req.DatedMemoryNotes)
	fmt.Fprintf(&b, "- latest_memory_note: %s\n", valueOrNone(req.LatestMemoryNote))
	fmt.Fprintf(&b, "- memory_validation_status: %s\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: %d\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: %d\n", req.SourceLines)
	b.WriteString("- review_pr_required: true\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_candidate_memory_included: false\n")
	b.WriteString("- raw_existing_memory_included: false\n")
	b.WriteString("- memory_file_written: false\n\n")
	fmt.Fprintf(&b, "Review this issue, then draft a compact memory change for `%s` on a normal code-review branch if the proposal is worth keeping. GitClaw does not write memory files from this issue.", req.Target.Path)
	return b.String()
}

func RenderMemoryProposalIssueActionReport(ev Event, req MemoryProposalIssueRequest, result MemoryProposalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Memory Proposal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_memory_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- memory_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- memory_proposal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- memory_proposal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- memory_proposal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- memory_proposal_id: `%s`\n", inlineCode(req.ProposalID))
	fmt.Fprintf(&b, "- normalized_target_kind: `%s`\n", req.Target.Kind)
	fmt.Fprintf(&b, "- normalized_target_path: `%s`\n", req.Target.Path)
	fmt.Fprintf(&b, "- target_present: `%t`\n", req.TargetPresent)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", valueOrNone(req.TargetSHA))
	fmt.Fprintf(&b, "- target_bytes: `%d`\n", req.TargetBytes)
	fmt.Fprintf(&b, "- target_lines: `%d`\n", req.TargetLines)
	fmt.Fprintf(&b, "- memory_budget_bytes: `%d`\n", req.MemoryBudget)
	fmt.Fprintf(&b, "- memory_budget_remaining_bytes: `%d`\n", req.RemainingBytes)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", req.DatedMemoryNotes)
	fmt.Fprintf(&b, "- latest_memory_note: `%s`\n", valueOrNone(req.LatestMemoryNote))
	fmt.Fprintf(&b, "- memory_validation_status: `%s`\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- memory_validation_errors: `%d`\n", req.ValidationErrors)
	fmt.Fprintf(&b, "- memory_validation_warnings: `%d`\n", req.ValidationWarnings)
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-memory-file")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_candidate_memory_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_existing_memory_included: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_memory_proposal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for a durable memory proposal. The issue is a review queue entry, not applied memory: `.gitclaw/MEMORY.md` and `.gitclaw/memory/*.md` are not written, candidate memory is not generated, and raw source request text is not copied into this receipt.\n\n")
	b.WriteString("### Review Path\n")
	fmt.Fprintf(&b, "- review proposal issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- if accepted, draft a compact memory diff for `%s` on a normal branch\n", req.Target.Path)
	b.WriteString("- run `gitclaw memory validate`, `gitclaw memory verify`, and a live GitHub Models conversation E2E before promotion\n")
	return strings.TrimSpace(b.String())
}

func parseMemoryProposalIssueArgs(args []string, sourceText string) (string, string, error) {
	target := "long-term"
	targetSet := false
	proposalID := ""
	for i := 0; i < len(args); i++ {
		field := strings.TrimSpace(args[i])
		if field == "" {
			continue
		}
		switch field {
		case "--target":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--target requires a value")
			}
			target = cleanMemoryPromoteTarget(args[i])
			targetSet = true
		case "--id":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--id requires a value")
			}
			proposalID = cleanMemoryProposalID(args[i])
		default:
			if strings.HasPrefix(field, "--") {
				return "", "", fmt.Errorf("unknown memory proposal argument %q", field)
			}
			if !targetSet {
				target = cleanMemoryPromoteTarget(field)
				targetSet = true
			}
		}
	}
	if proposalID == "" {
		proposalID = "memory-" + shortDocumentHash(sourceText)
	}
	return target, proposalID, nil
}

func cleanMemoryProposalID(id string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(id)), " \t\r\n.,:;!?`\"'")
}

func memoryProposalIssueMatches(body, proposalID string) bool {
	return strings.Contains(body, "<!-- "+memoryProposalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(proposalID)))
}

func valueOrNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}
