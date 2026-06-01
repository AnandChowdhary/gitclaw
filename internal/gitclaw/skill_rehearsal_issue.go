package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const skillRehearsalIssueMarker = "gitclaw:skill-rehearsal-issue"

type SkillRehearsalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type SkillRehearsalIssueRequest struct {
	Repo              string
	Command           string
	Subcommand        string
	RehearsalID       string
	RequestedSkill    string
	MatchedSkills     []SkillSummary
	MatchedSkillCount int
	AvailableSkills   int
	EnabledMatches    int
	DisabledMatches   int
	AllowlistBlocked  int
	MissingEnv        int
	MissingBins       int
	SelectedMatches   int
	SkillValidation   SkillValidationReport
	SourceIssueNumber int
	SourceCommentID   int64
	SourceSHA         string
	SourceBytes       int
	SourceLines       int
	SourceKind        string
}

type SkillRehearsalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsSkillRehearsalIssueRequest(ev Event, cfg Config) bool {
	return isSkillRehearsalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isSkillRehearsalIssueFields(fields []string) bool {
	if len(fields) < 2 || fields[0] != "/skills" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse", "rehearsal", "try", "trial", "practice":
		return true
	default:
		return false
	}
}

func BuildSkillRehearsalIssueRequest(ev Event, cfg Config, repoContext RepoContext) (SkillRehearsalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isSkillRehearsalIssueFields(fields) {
		return SkillRehearsalIssueRequest{}, fmt.Errorf("missing skill rehearsal issue command")
	}
	sourceText := activeRequestText(ev)
	skillName, rehearsalID, err := parseSkillRehearsalIssueArgs(fields[2:], sourceText)
	if err != nil {
		return SkillRehearsalIssueRequest{}, err
	}
	requestedSkill := strings.ToLower(cleanSkillLookupName(skillName))
	if requestedSkill == "" || !skillNamePattern.MatchString(requestedSkill) {
		return SkillRehearsalIssueRequest{}, fmt.Errorf("invalid skill rehearsal name %q", skillName)
	}
	if rehearsalID == "" {
		rehearsalID = fmt.Sprintf("skill-rehearsal-%s", shortDocumentHash(sourceText))
	}
	if !skillNamePattern.MatchString(rehearsalID) {
		return SkillRehearsalIssueRequest{}, fmt.Errorf("invalid skill rehearsal id %q", rehearsalID)
	}
	matches := matchingSkillSummaries(repoContext.SkillSummaries, requestedSkill)
	req := SkillRehearsalIssueRequest{
		Repo:              ev.Repo,
		Command:           fields[0],
		Subcommand:        strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RehearsalID:       rehearsalID,
		RequestedSkill:    requestedSkill,
		MatchedSkills:     append([]SkillSummary(nil), matches...),
		MatchedSkillCount: len(matches),
		AvailableSkills:   availableSkillCount(repoContext),
		SkillValidation:   ValidateSkillSummaries(repoContext.SkillSummaries),
		SourceIssueNumber: ev.Issue.Number,
		SourceSHA:         shortDocumentHash(sourceText),
		SourceBytes:       len(sourceText),
		SourceLines:       lineCount(sourceText),
		SourceKind:        "issue",
	}
	if ev.Comment != nil {
		req.SourceKind = "comment"
		req.SourceCommentID = ev.Comment.ID
	}
	for _, skill := range matches {
		if skillIsEnabled(skill) {
			req.EnabledMatches++
		}
		if skill.DisabledByConfig {
			req.DisabledMatches++
		}
		if skill.BlockedByAllowlist {
			req.AllowlistBlocked++
		}
		req.MissingEnv += len(skill.MissingEnv)
		req.MissingBins += len(skill.MissingBins)
		if skillSelectedForTurn(repoContext, skill) {
			req.SelectedMatches++
		}
	}
	return req, nil
}

func RunSkillRehearsalIssue(ctx context.Context, cfg Config, github SkillRehearsalIssueGitHubClient, req SkillRehearsalIssueRequest) (SkillRehearsalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return SkillRehearsalIssueResult{}, err
	}
	if req.RehearsalID == "" {
		return SkillRehearsalIssueResult{}, fmt.Errorf("missing skill rehearsal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return SkillRehearsalIssueResult{}, fmt.Errorf("list skill rehearsal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if skillRehearsalIssueMatches(issue.Body, req.RehearsalID) {
			return SkillRehearsalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, skillRehearsalIssueTitle(req), RenderSkillRehearsalIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return SkillRehearsalIssueResult{}, fmt.Errorf("create skill rehearsal issue: %w", err)
	}
	return SkillRehearsalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderSkillRehearsalIssueBody(req SkillRehearsalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" skill=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", skillRehearsalIssueMarker, escapeMarkerValue(req.RehearsalID), escapeMarkerValue(req.RequestedSkill), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw skill rehearsal issue.\n\n")
	fmt.Fprintf(&b, "- rehearsal_id: %s\n", req.RehearsalID)
	fmt.Fprintf(&b, "- requested_skill: %s\n", req.RequestedSkill)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- matched_skills: %d\n", req.MatchedSkillCount)
	fmt.Fprintf(&b, "- enabled_matches: %d\n", req.EnabledMatches)
	fmt.Fprintf(&b, "- disabled_matches: %d\n", req.DisabledMatches)
	fmt.Fprintf(&b, "- allowlist_blocked_matches: %d\n", req.AllowlistBlocked)
	fmt.Fprintf(&b, "- missing_env: %d\n", req.MissingEnv)
	fmt.Fprintf(&b, "- missing_bins: %d\n", req.MissingBins)
	fmt.Fprintf(&b, "- skill_validation_status: %s\n", req.SkillValidation.Status)
	b.WriteString("- rehearsal_mode: conversation-only\n")
	b.WriteString("- skill_install_allowed: false\n")
	b.WriteString("- skill_update_allowed: false\n")
	b.WriteString("- active_skill_write_performed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_skill_body_included: false\n\n")
	fmt.Fprintf(&b, "Use the `%s` skill when continuing this rehearsal. Keep any proposed skill changes in review-first issues or pull requests; this issue is only for trying the current prompt-visible behavior.\n", req.RequestedSkill)
	return strings.TrimSpace(b.String())
}

func RenderSkillRehearsalIssueActionReport(ev Event, req SkillRehearsalIssueRequest, result SkillRehearsalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Skill Rehearsal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_skill_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- skill_rehearsal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rehearsal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- rehearsal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- rehearsal_id_sha256_12: `%s`\n", shortDocumentHash(req.RehearsalID))
	fmt.Fprintf(&b, "- requested_skill: `%s`\n", inlineCode(req.RequestedSkill))
	fmt.Fprintf(&b, "- matched_skills: `%d`\n", req.MatchedSkillCount)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", req.AvailableSkills)
	fmt.Fprintf(&b, "- enabled_matches: `%d`\n", req.EnabledMatches)
	fmt.Fprintf(&b, "- disabled_matches: `%d`\n", req.DisabledMatches)
	fmt.Fprintf(&b, "- allowlist_blocked_matches: `%d`\n", req.AllowlistBlocked)
	fmt.Fprintf(&b, "- missing_env: `%d`\n", req.MissingEnv)
	fmt.Fprintf(&b, "- missing_bins: `%d`\n", req.MissingBins)
	fmt.Fprintf(&b, "- selected_matches_this_turn: `%d`\n", req.SelectedMatches)
	fmt.Fprintf(&b, "- skill_validation_status: `%s`\n", req.SkillValidation.Status)
	fmt.Fprintf(&b, "- skill_validation_errors: `%d`\n", req.SkillValidation.Errors)
	fmt.Fprintf(&b, "- skill_validation_warnings: `%d`\n", req.SkillValidation.Warnings)
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- active_skill_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_rehearsal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for trying a reviewed skill in a normal conversation. The action does not install skills, edit `SKILL.md`, fetch registries, or call a model; continue on the rehearsal issue to exercise the selected skill with GitHub Models.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- ask a normal `@gitclaw` follow-up that mentions `%s` or relies on the rehearsal issue body\n", inlineCode(req.RequestedSkill))
	b.WriteString("- verify the follow-up assistant marker includes prompt context, selected skill, prompt-visible tools, and usage telemetry\n")
	return strings.TrimSpace(b.String())
}

func parseSkillRehearsalIssueArgs(args []string, sourceText string) (string, string, error) {
	skillName := ""
	rehearsalID := ""
	for i := 0; i < len(args); i++ {
		field := strings.TrimSpace(args[i])
		if field == "" {
			continue
		}
		switch field {
		case "--id":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--id requires a value")
			}
			rehearsalID = cleanSkillRehearsalID(args[i])
		case "--skill":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--skill requires a value")
			}
			skillName = cleanSkillLookupName(args[i])
		default:
			if strings.HasPrefix(field, "--") {
				return "", "", fmt.Errorf("unknown skill rehearsal argument %q", field)
			}
			if skillName == "" {
				skillName = cleanSkillLookupName(field)
			}
		}
	}
	if skillName == "" {
		return "", "", fmt.Errorf("missing skill rehearsal name")
	}
	if rehearsalID == "" {
		rehearsalID = cleanSkillRehearsalID("skill-rehearsal-" + shortDocumentHash(sourceText))
	}
	return skillName, rehearsalID, nil
}

func cleanSkillRehearsalID(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	var b strings.Builder
	dash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '-' || r == '_' || r == '.':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
		if b.Len() >= 80 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func skillRehearsalIssueTitle(req SkillRehearsalIssueRequest) string {
	title := "GitClaw skill rehearsal: " + req.RequestedSkill
	if req.RehearsalID != "" {
		title += " (" + req.RehearsalID + ")"
	}
	return title
}

func skillRehearsalIssueMatches(body, rehearsalID string) bool {
	return strings.Contains(body, "<!-- "+skillRehearsalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(cleanSkillRehearsalID(rehearsalID))))
}
