package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const memoryRehearsalIssueMarker = "gitclaw:memory-rehearsal-issue"

type MemoryRehearsalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type MemoryRehearsalIssueRequest struct {
	Repo               string
	Command            string
	Subcommand         string
	RehearsalID        string
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

type MemoryRehearsalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsMemoryRehearsalIssueRequest(ev Event, cfg Config) bool {
	return isMemoryRehearsalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isMemoryRehearsalIssueFields(fields []string) bool {
	if len(fields) < 2 || (fields[0] != "/memory" && fields[0] != "/memories") {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse", "rehearsal", "try", "trial", "practice", "recall-test", "memory-test":
		return true
	default:
		return false
	}
}

func BuildMemoryRehearsalIssueRequest(ev Event, cfg Config, repoContext RepoContext) (MemoryRehearsalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isMemoryRehearsalIssueFields(fields) {
		return MemoryRehearsalIssueRequest{}, fmt.Errorf("missing memory rehearsal issue command")
	}
	sourceText := activeRequestText(ev)
	targetText, rehearsalID, err := parseMemoryRehearsalIssueArgs(fields[2:], sourceText)
	if err != nil {
		return MemoryRehearsalIssueRequest{}, err
	}
	target := normalizeMemoryPromoteTarget(targetText)
	if !target.Supported {
		return MemoryRehearsalIssueRequest{}, fmt.Errorf("unsupported memory rehearsal target %q", target.Requested)
	}
	if !skillNamePattern.MatchString(rehearsalID) {
		return MemoryRehearsalIssueRequest{}, fmt.Errorf("invalid memory rehearsal id %q", rehearsalID)
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
	return MemoryRehearsalIssueRequest{
		Repo:               ev.Repo,
		Command:            fields[0],
		Subcommand:         strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RehearsalID:        rehearsalID,
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

func RunMemoryRehearsalIssue(ctx context.Context, cfg Config, github MemoryRehearsalIssueGitHubClient, req MemoryRehearsalIssueRequest) (MemoryRehearsalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return MemoryRehearsalIssueResult{}, err
	}
	if req.RehearsalID == "" {
		return MemoryRehearsalIssueResult{}, fmt.Errorf("missing memory rehearsal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return MemoryRehearsalIssueResult{}, fmt.Errorf("list memory rehearsal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if memoryRehearsalIssueMatches(issue.Body, req.RehearsalID) {
			return MemoryRehearsalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, memoryRehearsalIssueTitle(req), RenderMemoryRehearsalIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return MemoryRehearsalIssueResult{}, fmt.Errorf("create memory rehearsal issue: %w", err)
	}
	return MemoryRehearsalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderMemoryRehearsalIssueBody(req MemoryRehearsalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" target_kind=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", memoryRehearsalIssueMarker, escapeMarkerValue(req.RehearsalID), escapeMarkerValue(req.Target.Kind), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw memory rehearsal issue.\n\n")
	fmt.Fprintf(&b, "- rehearsal_id: %s\n", req.RehearsalID)
	fmt.Fprintf(&b, "- target_kind: %s\n", req.Target.Kind)
	fmt.Fprintf(&b, "- target_path: %s\n", req.Target.Path)
	fmt.Fprintf(&b, "- target_present: %t\n", req.TargetPresent)
	fmt.Fprintf(&b, "- target_sha256_12: %s\n", valueOrNone(req.TargetSHA))
	fmt.Fprintf(&b, "- target_bytes: %d\n", req.TargetBytes)
	fmt.Fprintf(&b, "- target_lines: %d\n", req.TargetLines)
	fmt.Fprintf(&b, "- memory_budget_bytes: %d\n", req.MemoryBudget)
	fmt.Fprintf(&b, "- memory_budget_remaining_bytes: %d\n", req.RemainingBytes)
	fmt.Fprintf(&b, "- dated_memory_notes: %d\n", req.DatedMemoryNotes)
	fmt.Fprintf(&b, "- latest_memory_note: %s\n", valueOrNone(req.LatestMemoryNote))
	fmt.Fprintf(&b, "- memory_validation_status: %s\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: %d\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: %d\n", req.SourceLines)
	b.WriteString("- rehearsal_mode: github-issue-conversation\n")
	b.WriteString("- memory_write_allowed: false\n")
	b.WriteString("- candidate_memory_generation_allowed: false\n")
	b.WriteString("- repository_mutation_allowed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_target_memory_included: false\n")
	b.WriteString("- raw_candidate_memory_included: false\n\n")
	fmt.Fprintf(&b, "Use this issue to rehearse the current `%s` behavior in a normal GitClaw conversation. Proposed memory changes belong in `/memory remember` or a reviewed pull request; this issue is only for trying the current prompt-visible behavior.\n", req.Target.Path)
	return strings.TrimSpace(b.String())
}

func RenderMemoryRehearsalIssueActionReport(ev Event, req MemoryRehearsalIssueRequest, result MemoryRehearsalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Memory Rehearsal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_memory_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- memory_rehearsal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rehearsal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- rehearsal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- rehearsal_id_sha256_12: `%s`\n", shortDocumentHash(req.RehearsalID))
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
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- candidate_memory_generation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_memory_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_candidate_memory_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_memory_rehearsal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for rehearsing the current memory context in a normal conversation. The action does not generate candidate memory, write `.gitclaw/` files, mutate the repository, or call a model; continue on the rehearsal issue to exercise current memory behavior with GitHub Models.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- ask a normal `@gitclaw` follow-up that relies on `%s` without requesting a memory edit\n", req.Target.Path)
	b.WriteString("- verify the follow-up assistant marker includes prompt context, selected skill, prompt-visible tools, and usage telemetry\n")
	return strings.TrimSpace(b.String())
}

func parseMemoryRehearsalIssueArgs(args []string, sourceText string) (string, string, error) {
	target := "long-term"
	targetSet := false
	rehearsalID := ""
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
			rehearsalID = cleanMemoryRehearsalID(args[i])
		default:
			if strings.HasPrefix(field, "--") {
				return "", "", fmt.Errorf("unknown memory rehearsal argument %q", field)
			}
			if !targetSet {
				target = cleanMemoryPromoteTarget(field)
				targetSet = true
			}
		}
	}
	if rehearsalID == "" {
		rehearsalID = cleanMemoryRehearsalID("memory-rehearsal-" + shortDocumentHash(sourceText))
	}
	return target, rehearsalID, nil
}

func cleanMemoryRehearsalID(value string) string {
	return cleanSkillRehearsalID(value)
}

func memoryRehearsalIssueTitle(req MemoryRehearsalIssueRequest) string {
	title := "GitClaw memory rehearsal: " + req.Target.Path
	if req.RehearsalID != "" {
		title += " (" + req.RehearsalID + ")"
	}
	return title
}

func memoryRehearsalIssueMatches(body, rehearsalID string) bool {
	return strings.Contains(body, "<!-- "+memoryRehearsalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(cleanMemoryRehearsalID(rehearsalID))))
}
