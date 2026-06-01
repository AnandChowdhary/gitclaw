package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const skillBundleRehearsalIssueMarker = "gitclaw:bundle-rehearsal-issue"

type SkillBundleRehearsalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type SkillBundleRehearsalIssueRequest struct {
	Repo                  string
	Command               string
	Subcommand            string
	RehearsalID           string
	RequestedBundle       string
	MatchedBundles        []SkillBundleSummary
	MatchedBundleCount    int
	AvailableBundles      int
	AvailableSkills       int
	SelectedMatches       int
	BundlePath            string
	BundleSHA             string
	BundleBytes           int
	BundleLines           int
	BundleSkillRefs       int
	ResolvedSkills        []string
	MissingSkills         []string
	ResolvedBundleSkills  int
	MissingBundleSkills   int
	InstructionPresent    bool
	InstructionSHA        string
	BundleRiskFindings    int
	BundleRiskMaxSeverity string
	BundleRiskCodes       []string
	SourceIssueNumber     int
	SourceCommentID       int64
	SourceSHA             string
	SourceBytes           int
	SourceLines           int
	SourceKind            string
}

type SkillBundleRehearsalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsSkillBundleRehearsalIssueRequest(ev Event, cfg Config) bool {
	return isSkillBundleRehearsalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isSkillBundleRehearsalIssueFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	switch fields[0] {
	case "/bundles", "/bundle":
	default:
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse", "rehearsal", "try", "trial", "practice", "workshop":
		return true
	default:
		return false
	}
}

func BuildSkillBundleRehearsalIssueRequest(ev Event, cfg Config, repoContext RepoContext) (SkillBundleRehearsalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isSkillBundleRehearsalIssueFields(fields) {
		return SkillBundleRehearsalIssueRequest{}, fmt.Errorf("missing skill bundle rehearsal issue command")
	}
	sourceText := activeRequestText(ev)
	bundleName, rehearsalID, err := parseSkillBundleRehearsalIssueArgs(fields[2:], sourceText)
	if err != nil {
		return SkillBundleRehearsalIssueRequest{}, err
	}
	requestedBundle := normalizeSkillBundleName(cleanSkillLookupName(bundleName))
	if requestedBundle == "" || !skillNamePattern.MatchString(requestedBundle) {
		return SkillBundleRehearsalIssueRequest{}, fmt.Errorf("invalid skill bundle rehearsal name %q", bundleName)
	}
	if rehearsalID == "" {
		rehearsalID = cleanSkillRehearsalID("bundle-rehearsal-" + shortDocumentHash(sourceText))
	}
	if !skillNamePattern.MatchString(rehearsalID) {
		return SkillBundleRehearsalIssueRequest{}, fmt.Errorf("invalid skill bundle rehearsal id %q", rehearsalID)
	}
	matches := matchingSkillBundleSummaries(repoContext.SkillBundles, requestedBundle)
	req := SkillBundleRehearsalIssueRequest{
		Repo:               ev.Repo,
		Command:            fields[0],
		Subcommand:         strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RehearsalID:        rehearsalID,
		RequestedBundle:    requestedBundle,
		MatchedBundles:     append([]SkillBundleSummary(nil), matches...),
		MatchedBundleCount: len(matches),
		AvailableBundles:   len(repoContext.SkillBundles),
		AvailableSkills:    availableSkillCount(repoContext),
		SourceIssueNumber:  ev.Issue.Number,
		SourceSHA:          shortDocumentHash(sourceText),
		SourceBytes:        len(sourceText),
		SourceLines:        lineCount(sourceText),
		SourceKind:         "issue",
	}
	if ev.Comment != nil {
		req.SourceKind = "comment"
		req.SourceCommentID = ev.Comment.ID
	}
	for _, bundle := range matches {
		if bundle.Selected {
			req.SelectedMatches++
		}
	}
	if len(matches) > 0 {
		primary := matches[0]
		req.BundlePath = primary.Path
		req.BundleSHA = primary.SHA
		req.BundleBytes = primary.Bytes
		req.BundleLines = primary.Lines
		req.BundleSkillRefs = len(primary.Skills)
		req.ResolvedSkills = append([]string(nil), primary.ResolvedSkills...)
		req.MissingSkills = append([]string(nil), primary.MissingSkills...)
		req.ResolvedBundleSkills = len(primary.ResolvedSkills)
		req.MissingBundleSkills = len(primary.MissingSkills)
		req.InstructionPresent = primary.InstructionPresent
		req.InstructionSHA = primary.InstructionSHA
		req.BundleRiskFindings = len(primary.RiskFindings)
		req.BundleRiskMaxSeverity = skillBundleRiskMaxSeverity(primary.RiskFindings)
		req.BundleRiskCodes = skillBundleRiskCodes(primary.RiskFindings)
	}
	return req, nil
}

func RunSkillBundleRehearsalIssue(ctx context.Context, cfg Config, github SkillBundleRehearsalIssueGitHubClient, req SkillBundleRehearsalIssueRequest) (SkillBundleRehearsalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return SkillBundleRehearsalIssueResult{}, err
	}
	if req.RehearsalID == "" {
		return SkillBundleRehearsalIssueResult{}, fmt.Errorf("missing skill bundle rehearsal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return SkillBundleRehearsalIssueResult{}, fmt.Errorf("list skill bundle rehearsal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if skillBundleRehearsalIssueMatches(issue.Body, req.RehearsalID) {
			return SkillBundleRehearsalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, skillBundleRehearsalIssueTitle(req), RenderSkillBundleRehearsalIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return SkillBundleRehearsalIssueResult{}, fmt.Errorf("create skill bundle rehearsal issue: %w", err)
	}
	return SkillBundleRehearsalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderSkillBundleRehearsalIssueBody(req SkillBundleRehearsalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" bundle=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", skillBundleRehearsalIssueMarker, escapeMarkerValue(req.RehearsalID), escapeMarkerValue(req.RequestedBundle), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw skill bundle rehearsal issue.\n\n")
	fmt.Fprintf(&b, "- rehearsal_id: %s\n", req.RehearsalID)
	fmt.Fprintf(&b, "- requested_bundle: %s\n", req.RequestedBundle)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- matched_bundles: %d\n", req.MatchedBundleCount)
	fmt.Fprintf(&b, "- available_bundles: %d\n", req.AvailableBundles)
	fmt.Fprintf(&b, "- available_skills: %d\n", req.AvailableSkills)
	fmt.Fprintf(&b, "- bundle_path: %s\n", noneIfEmpty(req.BundlePath))
	fmt.Fprintf(&b, "- bundle_sha256_12: %s\n", noneIfEmpty(req.BundleSHA))
	fmt.Fprintf(&b, "- bundle_bytes: %d\n", req.BundleBytes)
	fmt.Fprintf(&b, "- bundle_lines: %d\n", req.BundleLines)
	fmt.Fprintf(&b, "- bundle_skill_refs: %d\n", req.BundleSkillRefs)
	fmt.Fprintf(&b, "- resolved_bundle_skills: %d\n", req.ResolvedBundleSkills)
	fmt.Fprintf(&b, "- resolved_skills: %s\n", inlineListOrNone(req.ResolvedSkills))
	fmt.Fprintf(&b, "- missing_bundle_skills: %d\n", req.MissingBundleSkills)
	fmt.Fprintf(&b, "- missing_skills: %s\n", inlineListOrNone(req.MissingSkills))
	fmt.Fprintf(&b, "- instruction_present: %t\n", req.InstructionPresent)
	fmt.Fprintf(&b, "- instruction_sha256_12: %s\n", noneIfEmpty(req.InstructionSHA))
	fmt.Fprintf(&b, "- bundle_risk_findings: %d\n", req.BundleRiskFindings)
	fmt.Fprintf(&b, "- bundle_risk_max_severity: %s\n", noneIfEmpty(req.BundleRiskMaxSeverity))
	fmt.Fprintf(&b, "- bundle_risk_codes: %s\n", inlineListOrNone(req.BundleRiskCodes))
	b.WriteString("- rehearsal_mode: github-issue-conversation\n")
	b.WriteString("- bundle_update_allowed: false\n")
	b.WriteString("- skill_install_allowed: false\n")
	b.WriteString("- skill_update_allowed: false\n")
	b.WriteString("- active_skill_write_performed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_bundle_body_included: false\n")
	b.WriteString("- raw_bundle_instruction_included: false\n")
	b.WriteString("- raw_skill_bodies_included: false\n\n")
	fmt.Fprintf(&b, "Use the `%s` bundle when continuing this rehearsal. Treat it as a task profile over reviewed repo-local skills; keep any proposed bundle or skill changes in review-first issues or pull requests.\n", req.RequestedBundle)
	return strings.TrimSpace(b.String())
}

func RenderSkillBundleRehearsalIssueActionReport(ev Event, req SkillBundleRehearsalIssueRequest, result SkillBundleRehearsalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Skill Bundle Rehearsal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_bundle_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- bundle_rehearsal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rehearsal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- rehearsal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- rehearsal_id_sha256_12: `%s`\n", shortDocumentHash(req.RehearsalID))
	fmt.Fprintf(&b, "- requested_bundle: `%s`\n", inlineCode(req.RequestedBundle))
	fmt.Fprintf(&b, "- matched_bundles: `%d`\n", req.MatchedBundleCount)
	fmt.Fprintf(&b, "- available_bundles: `%d`\n", req.AvailableBundles)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", req.AvailableSkills)
	fmt.Fprintf(&b, "- selected_matches_this_turn: `%d`\n", req.SelectedMatches)
	fmt.Fprintf(&b, "- bundle_path: `%s`\n", noneIfEmpty(req.BundlePath))
	fmt.Fprintf(&b, "- bundle_sha256_12: `%s`\n", noneIfEmpty(req.BundleSHA))
	fmt.Fprintf(&b, "- bundle_skill_refs: `%d`\n", req.BundleSkillRefs)
	fmt.Fprintf(&b, "- resolved_bundle_skills: `%d`\n", req.ResolvedBundleSkills)
	fmt.Fprintf(&b, "- missing_bundle_skills: `%d`\n", req.MissingBundleSkills)
	fmt.Fprintf(&b, "- instruction_present: `%t`\n", req.InstructionPresent)
	fmt.Fprintf(&b, "- instruction_sha256_12: `%s`\n", noneIfEmpty(req.InstructionSHA))
	fmt.Fprintf(&b, "- bundle_risk_findings: `%d`\n", req.BundleRiskFindings)
	fmt.Fprintf(&b, "- bundle_risk_max_severity: `%s`\n", noneIfEmpty(req.BundleRiskMaxSeverity))
	fmt.Fprintf(&b, "- bundle_risk_codes: `%s`\n", inlineListOrNone(req.BundleRiskCodes))
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- bundle_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- active_skill_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_instruction_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_bundle_rehearsal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for trying a reviewed skill bundle as a normal conversation lane. The action does not install skills, edit bundle YAML, fetch registries, or call a model; continue on the rehearsal issue to exercise the bundle with GitHub Models.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- ask a normal `@gitclaw` follow-up that mentions the `%s` bundle or one of its resolved skills\n", inlineCode(req.RequestedBundle))
	b.WriteString("- verify the follow-up assistant marker includes prompt context, selected skill, prompt-visible tools, and usage telemetry\n")
	return strings.TrimSpace(b.String())
}

func parseSkillBundleRehearsalIssueArgs(args []string, sourceText string) (string, string, error) {
	bundleName := ""
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
		case "--bundle", "--name":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("%s requires a value", field)
			}
			bundleName = normalizeSkillBundleName(cleanSkillLookupName(args[i]))
		default:
			if strings.HasPrefix(field, "--") {
				return "", "", fmt.Errorf("unknown skill bundle rehearsal argument %q", field)
			}
			if bundleName == "" {
				bundleName = normalizeSkillBundleName(cleanSkillLookupName(field))
			}
		}
	}
	if bundleName == "" {
		return "", "", fmt.Errorf("missing skill bundle rehearsal name")
	}
	if rehearsalID == "" {
		rehearsalID = cleanSkillRehearsalID("bundle-rehearsal-" + shortDocumentHash(sourceText))
	}
	return bundleName, rehearsalID, nil
}

func skillBundleRehearsalIssueTitle(req SkillBundleRehearsalIssueRequest) string {
	title := "GitClaw bundle rehearsal: " + req.RequestedBundle
	if req.RehearsalID != "" {
		title += " (" + req.RehearsalID + ")"
	}
	return title
}

func skillBundleRehearsalIssueMatches(body, rehearsalID string) bool {
	return strings.Contains(body, "<!-- "+skillBundleRehearsalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(cleanSkillRehearsalID(rehearsalID))))
}
