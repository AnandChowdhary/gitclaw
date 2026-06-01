package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const skillProposalIssueMarker = "gitclaw:skill-proposal-issue"

type SkillProposalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type SkillProposalIssueRequest struct {
	Repo                 string
	Command              string
	Subcommand           string
	RequestedAction      string
	PlannedAction        string
	Target               skillInstallPlanTarget
	ProposalPath         string
	DestinationPath      string
	ExistingSkillMatches int
	AvailableSkills      int
	SourceIssueNumber    int
	SourceCommentID      int64
	SourceSHA            string
	SourceBytes          int
	SourceLines          int
	SourceKind           string
}

type SkillProposalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsSkillProposalIssueRequest(ev Event, cfg Config) bool {
	return isSkillProposalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isSkillProposalIssueFields(fields []string) bool {
	if len(fields) < 2 || fields[0] != "/skills" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose", "proposal", "proposal-issue", "propose-create", "propose-update":
		return true
	default:
		return false
	}
}

func BuildSkillProposalIssueRequest(ev Event, cfg Config, repoContext RepoContext) (SkillProposalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isSkillProposalIssueFields(fields) {
		return SkillProposalIssueRequest{}, fmt.Errorf("missing skill proposal issue command")
	}
	if len(fields) < 3 {
		return SkillProposalIssueRequest{}, fmt.Errorf("missing skill proposal target")
	}
	requestedAction := "auto"
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose-create":
		requestedAction = "propose-create"
	case "propose-update":
		requestedAction = "propose-update"
	}
	target := classifySkillInstallTarget(fields[2])
	if target.Candidate == "" {
		return SkillProposalIssueRequest{}, fmt.Errorf("missing safe skill proposal name")
	}
	if !skillNamePattern.MatchString(target.Candidate) {
		return SkillProposalIssueRequest{}, fmt.Errorf("invalid safe skill proposal name %q", target.Candidate)
	}
	matches := matchingInstallPlanSkillSummaries(repoContext.SkillSummaries, target)
	sourceText := activeRequestText(ev)
	sourceKind := "issue"
	var sourceCommentID int64
	if ev.Comment != nil {
		sourceKind = "comment"
		sourceCommentID = ev.Comment.ID
	}
	return SkillProposalIssueRequest{
		Repo:                 ev.Repo,
		Command:              fields[0],
		Subcommand:           strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RequestedAction:      requestedAction,
		PlannedAction:        plannedSkillProposalAction(requestedAction, len(matches)),
		Target:               target,
		ProposalPath:         skillProposalPlanPath(target.Candidate),
		DestinationPath:      skillInstallDestinationPath(target.Candidate),
		ExistingSkillMatches: len(matches),
		AvailableSkills:      availableSkillCount(repoContext),
		SourceIssueNumber:    ev.Issue.Number,
		SourceCommentID:      sourceCommentID,
		SourceSHA:            shortDocumentHash(sourceText),
		SourceBytes:          len(sourceText),
		SourceLines:          lineCount(sourceText),
		SourceKind:           sourceKind,
	}, nil
}

func RunSkillProposalIssue(ctx context.Context, github SkillProposalIssueGitHubClient, req SkillProposalIssueRequest) (SkillProposalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return SkillProposalIssueResult{}, err
	}
	if req.Target.Candidate == "" {
		return SkillProposalIssueResult{}, fmt.Errorf("missing skill proposal name")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, nil, 300)
	if err != nil {
		return SkillProposalIssueResult{}, fmt.Errorf("list skill proposal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if skillProposalIssueMatches(issue.Body, req.Target.Candidate) {
			return SkillProposalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	title := fmt.Sprintf("GitClaw skill proposal: %s", req.Target.Candidate)
	issue, err := github.CreateIssue(ctx, req.Repo, title, RenderSkillProposalIssueBody(req), nil)
	if err != nil {
		return SkillProposalIssueResult{}, fmt.Errorf("create skill proposal issue: %w", err)
	}
	return SkillProposalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderSkillProposalIssueBody(req SkillProposalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s name=\"%s\" planned_action=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", skillProposalIssueMarker, escapeMarkerValue(req.Target.Candidate), escapeMarkerValue(req.PlannedAction), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw skill proposal issue.\n\n")
	fmt.Fprintf(&b, "- proposal_name: %s\n", req.Target.Candidate)
	fmt.Fprintf(&b, "- planned_action: %s\n", req.PlannedAction)
	fmt.Fprintf(&b, "- requested_action: %s\n", req.RequestedAction)
	fmt.Fprintf(&b, "- proposal_path: %s\n", req.ProposalPath)
	fmt.Fprintf(&b, "- destination_path: %s\n", req.DestinationPath)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: %d\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: %d\n", req.SourceLines)
	fmt.Fprintf(&b, "- existing_skill_matches: %d\n", req.ExistingSkillMatches)
	fmt.Fprintf(&b, "- available_skills: %d\n", req.AvailableSkills)
	b.WriteString("- review_pr_required: true\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_proposal_body_included: false\n")
	b.WriteString("- active_skill_write_performed: false\n\n")
	fmt.Fprintf(&b, "Review this issue, then draft `%s` on a normal code-review branch if the proposal is worth keeping. GitClaw does not write or apply active skills from this issue.", req.ProposalPath)
	return b.String()
}

func RenderSkillProposalIssueActionReport(ev Event, req SkillProposalIssueRequest, result SkillProposalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Skill Proposal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_skill_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- proposal_issue_status: `%s`\n", status)
	fmt.Fprintf(&b, "- proposal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- proposal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- proposal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- requested_action: `%s`\n", req.RequestedAction)
	fmt.Fprintf(&b, "- planned_proposal_action: `%s`\n", req.PlannedAction)
	fmt.Fprintf(&b, "- target_type: `%s`\n", req.Target.Type)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", req.Target.Hash)
	fmt.Fprintf(&b, "- target_terms: `%d`\n", req.Target.Terms)
	fmt.Fprintf(&b, "- safe_name_candidate: `%s`\n", inlineCode(req.Target.Candidate))
	fmt.Fprintf(&b, "- proposal_path: `%s`\n", req.ProposalPath)
	fmt.Fprintf(&b, "- destination_path: `%s`\n", req.DestinationPath)
	fmt.Fprintf(&b, "- existing_skill_matches: `%d`\n", req.ExistingSkillMatches)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", req.AvailableSkills)
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-proposal-file")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_proposal_issue_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- proposal_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- active_skill_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_proposal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for a reusable skill proposal. The issue is a review queue entry, not an applied skill: active `SKILL.md` files are not written, proposal files are not written, and raw source request text is not copied into this receipt.\n\n")
	b.WriteString("### Review Path\n")
	fmt.Fprintf(&b, "- review proposal issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- if accepted, draft `%s` on a normal branch and review it as code\n", req.ProposalPath)
	fmt.Fprintf(&b, "- only after review should the proposal become `%s`\n", req.DestinationPath)
	return strings.TrimSpace(b.String())
}

func skillProposalIssueMatches(body, name string) bool {
	return strings.Contains(body, "<!-- "+skillProposalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`name="%s"`, escapeMarkerValue(name)))
}
