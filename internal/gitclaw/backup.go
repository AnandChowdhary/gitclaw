package gitclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type IssueBackup struct {
	Version     int                  `json:"version"`
	GeneratedAt string               `json:"generated_at"`
	Repo        string               `json:"repo"`
	EventName   string               `json:"event_name"`
	Issue       IssueBackupIssue     `json:"issue"`
	Transcript  []TranscriptMessage  `json:"transcript"`
	Comments    []IssueBackupComment `json:"comments"`
}

type IssueBackupIssue struct {
	Number            int      `json:"number"`
	Title             string   `json:"title"`
	Body              string   `json:"body"`
	Author            string   `json:"author"`
	AuthorAssociation string   `json:"author_association"`
	Labels            []string `json:"labels"`
}

type IssueBackupComment struct {
	ID                int64  `json:"id"`
	Body              string `json:"body"`
	Author            string `json:"author"`
	AuthorAssociation string `json:"author_association"`
	CreatedAt         string `json:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

type BackupIndex struct {
	Version     int                `json:"version"`
	GeneratedAt string             `json:"generated_at"`
	Repo        string             `json:"repo"`
	Count       int                `json:"count"`
	Issues      []BackupIndexIssue `json:"issues"`
}

type BackupIndexIssue struct {
	Number             int      `json:"number"`
	Title              string   `json:"title"`
	Path               string   `json:"path"`
	BackupGeneratedAt  string   `json:"backup_generated_at"`
	EventName          string   `json:"event_name"`
	Labels             []string `json:"labels,omitempty"`
	CommentCount       int      `json:"comment_count"`
	TranscriptMessages int      `json:"transcript_messages"`
}

type BackupVerifyResult struct {
	Root                 string
	Repo                 string
	RepoDir              string
	IndexPath            string
	ReadmePath           string
	IssuesChecked        int
	CommentsChecked      int
	TranscriptMessages   int
	UnindexedIssueFiles  int
	VerificationFailures []string
}

type BackupJSONLRecord struct {
	Schema            string `json:"schema"`
	Repo              string `json:"repo"`
	IssueNumber       int    `json:"issue_number"`
	IssueTitle        string `json:"issue_title"`
	BackupGeneratedAt string `json:"backup_generated_at"`
	EventName         string `json:"event_name"`
	Sequence          int    `json:"sequence"`
	Source            string `json:"source"`
	Role              string `json:"role"`
	Actor             string `json:"actor"`
	AuthorAssociation string `json:"author_association"`
	CommentID         int64  `json:"comment_id,omitempty"`
	Edited            bool   `json:"edited"`
	Trusted           bool   `json:"trusted"`
	BodySHA           string `json:"body_sha256_12"`
	Body              string `json:"body"`
}

type BackupRestorePlan struct {
	Root                  string
	Repo                  string
	TargetRepo            string
	IssuePath             string
	IssueNumber           int
	BackupGeneratedAt     string
	EventName             string
	SchemaVersion         int
	Labels                []string
	Comments              int
	TranscriptMessages    int
	AssistantTurns        int
	ErrorComments         int
	UserMessages          int
	AssistantMessages     int
	IssueTitleSHA         string
	IssueBodySHA          string
	CommentBodySHAs       []string
	TranscriptMessageSHAs []string
}

func (r BackupVerifyResult) OK() bool {
	return len(r.VerificationFailures) == 0
}

func BackupIssue(ctx context.Context, ev Event, github GitHubClient, outDir string, generatedAt time.Time) (string, error) {
	if err := validateRepoName(ev.Repo); err != nil {
		return "", err
	}
	if outDir == "" {
		outDir = filepath.Join(".gitclaw", "backups")
	}
	comments, err := github.ListIssueComments(ctx, ev.Repo, ev.Issue.Number)
	if err != nil {
		return "", fmt.Errorf("list issue comments: %w", err)
	}
	backup := IssueBackup{
		Version:     1,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Repo:        ev.Repo,
		EventName:   ev.EventName,
		Issue: IssueBackupIssue{
			Number:            ev.Issue.Number,
			Title:             ev.Issue.Title,
			Body:              ev.Issue.Body,
			Author:            ev.Issue.User.Login,
			AuthorAssociation: ev.Issue.AuthorAssociation,
			Labels:            append([]string(nil), ev.Issue.Labels...),
		},
		Transcript: BuildTranscript(ev, comments),
		Comments:   backupComments(comments),
	}
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	path := issueBackupPath(outDir, ev.Repo, ev.Issue.Number)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func VerifyBackupTree(root, repo string) (BackupVerifyResult, error) {
	if err := validateRepoName(repo); err != nil {
		return BackupVerifyResult{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	repoDir := backupRepoDir(root, repo)
	result := BackupVerifyResult{
		Root:       filepath.ToSlash(root),
		Repo:       repo,
		RepoDir:    filepath.ToSlash(repoDir),
		IndexPath:  filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath: filepath.ToSlash(filepath.Join(repoDir, "README.md")),
	}

	indexData, err := os.ReadFile(filepath.Join(repoDir, "index.json"))
	if err != nil {
		result.addFailure("index_json_readable", err.Error())
		return result, nil
	}
	var index BackupIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		result.addFailure("index_json_valid", err.Error())
		return result, nil
	}
	if index.Version != 1 {
		result.addFailure("index_version", fmt.Sprintf("got %d, want 1", index.Version))
	}
	if index.Repo != repo {
		result.addFailure("index_repo", fmt.Sprintf("got %q, want %q", index.Repo, repo))
	}
	if index.Count != len(index.Issues) {
		result.addFailure("index_count", fmt.Sprintf("got %d, want %d", index.Count, len(index.Issues)))
	}
	if _, err := time.Parse(time.RFC3339, index.GeneratedAt); err != nil {
		result.addFailure("index_generated_at", "not RFC3339")
	}
	if _, err := os.Stat(filepath.Join(repoDir, "README.md")); err != nil {
		result.addFailure("readme_present", err.Error())
	}

	seenNumbers := map[int]bool{}
	seenPaths := map[string]bool{}
	lastNumber := -1
	for _, issue := range index.Issues {
		result.verifyIndexIssue(repoDir, repo, issue, seenNumbers, seenPaths, &lastNumber)
	}
	result.verifyNoUnindexedIssues(repoDir, seenPaths)
	return result, nil
}

func WriteBackupIndex(root, repo string, generatedAt time.Time) (string, error) {
	if err := validateRepoName(repo); err != nil {
		return "", err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	repoDir := backupRepoDir(root, repo)
	matches, err := filepath.Glob(filepath.Join(repoDir, "issues", "*.json"))
	if err != nil {
		return "", err
	}
	issues := make([]BackupIndexIssue, 0, len(matches))
	for _, match := range matches {
		issue, err := readBackupIndexIssue(repoDir, match)
		if err != nil {
			continue
		}
		issues = append(issues, issue)
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	index := BackupIndex{
		Version:     1,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Repo:        repo,
		Count:       len(issues),
		Issues:      issues,
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return "", err
	}
	indexPath := filepath.Join(repoDir, "index.json")
	if err := os.WriteFile(indexPath, data, 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte(RenderBackupIndexReadme(index)), 0o600); err != nil {
		return "", err
	}
	return indexPath, nil
}

func ExportBackupJSONL(root, repo string, issueNumber int) (string, error) {
	if err := validateRepoName(repo); err != nil {
		return "", err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	matched := 0
	for _, issue := range index.Issues {
		if issueNumber > 0 && issue.Number != issueNumber {
			continue
		}
		backup, err := readIndexedBackup(repoDir, repo, issue)
		if err != nil {
			return "", err
		}
		matched++
		for i, msg := range backup.Transcript {
			line, err := json.Marshal(backupJSONLRecord(backup, msg, i+1))
			if err != nil {
				return "", err
			}
			b.Write(line)
			b.WriteByte('\n')
		}
	}
	if issueNumber > 0 && matched == 0 {
		return "", fmt.Errorf("issue #%d not found in backup index", issueNumber)
	}
	return b.String(), nil
}

func PlanBackupRestore(root, repo string, issueNumber int, targetRepo string) (BackupRestorePlan, error) {
	if issueNumber <= 0 {
		return BackupRestorePlan{}, fmt.Errorf("missing positive issue number")
	}
	if err := validateRepoName(repo); err != nil {
		return BackupRestorePlan{}, err
	}
	if targetRepo == "" {
		targetRepo = repo
	}
	if err := validateRepoName(targetRepo); err != nil {
		return BackupRestorePlan{}, err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupRestorePlan{}, err
	}
	for _, issue := range index.Issues {
		if issue.Number != issueNumber {
			continue
		}
		backup, err := readIndexedBackup(repoDir, repo, issue)
		if err != nil {
			return BackupRestorePlan{}, err
		}
		return buildBackupRestorePlan(root, repo, targetRepo, issue, backup), nil
	}
	return BackupRestorePlan{}, fmt.Errorf("issue #%d not found in backup index", issueNumber)
}

func RenderBackupRestorePlan(plan BackupRestorePlan) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Restore Plan\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- restore_mode: `%s`\n", "dry-run")
	fmt.Fprintf(&b, "- source_repository: `%s`\n", plan.Repo)
	fmt.Fprintf(&b, "- target_repository: `%s`\n", plan.TargetRepo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", plan.IssueNumber)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", filepath.ToSlash(plan.Root))
	fmt.Fprintf(&b, "- issue_backup_path: `%s`\n", plan.IssuePath)
	fmt.Fprintf(&b, "- backup_generated_at: `%s`\n", plan.BackupGeneratedAt)
	fmt.Fprintf(&b, "- backup_event_name: `%s`\n", plan.EventName)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", plan.SchemaVersion)
	fmt.Fprintf(&b, "- labels: `%d`\n", len(plan.Labels))
	fmt.Fprintf(&b, "- comments: `%d`\n", plan.Comments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", plan.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", plan.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", plan.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", plan.AssistantTurns)
	fmt.Fprintf(&b, "- error_comments: `%d`\n", plan.ErrorComments)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", plan.IssueTitleSHA)
	fmt.Fprintf(&b, "- issue_body_sha256_12: `%s`\n", plan.IssueBodySHA)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n\n", false)
	b.WriteString("This is a non-mutating recovery plan. It reads the local backup tree only; it does not create issues, post comments, apply labels, or call GitHub APIs.\n\n")
	b.WriteString("### Planned Restore Actions\n")
	b.WriteString("- create one new issue in the target repository using the backed-up title and body\n")
	b.WriteString("- apply backed-up labels that still exist or can be created by a future approved restore command\n")
	b.WriteString("- replay backed-up comments in backup order as the restoring actor, preserving original author/comment metadata in comment headers\n")
	b.WriteString("- preserve assistant-turn and error marker bodies from the raw backup if a future approved restore command is allowed to post them\n")
	b.WriteString("- verify the new issue transcript against the hashes below before considering restore complete\n\n")
	b.WriteString("### Labels\n")
	if len(plan.Labels) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, label := range plan.Labels {
			fmt.Fprintf(&b, "- `%s`\n", inlineCode(label))
		}
	}
	b.WriteString("\n### Comment Body Hashes\n")
	writeHashList(&b, "comment", plan.CommentBodySHAs)
	b.WriteString("\n### Transcript Body Hashes\n")
	writeHashList(&b, "message", plan.TranscriptMessageSHAs)
	return strings.TrimSpace(b.String())
}

func readBackupIndex(root, repo string) (string, BackupIndex, error) {
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	repoDir := backupRepoDir(root, repo)
	indexData, err := os.ReadFile(filepath.Join(repoDir, "index.json"))
	if err != nil {
		return "", BackupIndex{}, fmt.Errorf("read backup index: %w", err)
	}
	var index BackupIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		return "", BackupIndex{}, fmt.Errorf("parse backup index: %w", err)
	}
	if index.Repo != repo {
		return "", BackupIndex{}, fmt.Errorf("backup index repo %q does not match %q", index.Repo, repo)
	}
	return repoDir, index, nil
}

func readIndexedBackup(repoDir, repo string, issue BackupIndexIssue) (IssueBackup, error) {
	absPath, err := safeBackupPayloadPath(repoDir, issue.Path)
	if err != nil {
		return IssueBackup{}, err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return IssueBackup{}, fmt.Errorf("read issue backup %s: %w", issue.Path, err)
	}
	var backup IssueBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return IssueBackup{}, fmt.Errorf("parse issue backup %s: %w", issue.Path, err)
	}
	if backup.Repo != repo {
		return IssueBackup{}, fmt.Errorf("issue backup %s repo %q does not match %q", issue.Path, backup.Repo, repo)
	}
	if backup.Issue.Number != issue.Number {
		return IssueBackup{}, fmt.Errorf("issue backup %s number %d does not match index number %d", issue.Path, backup.Issue.Number, issue.Number)
	}
	return backup, nil
}

func buildBackupRestorePlan(root, repo, targetRepo string, issue BackupIndexIssue, backup IssueBackup) BackupRestorePlan {
	plan := BackupRestorePlan{
		Root:               root,
		Repo:               repo,
		TargetRepo:         targetRepo,
		IssuePath:          issue.Path,
		IssueNumber:        backup.Issue.Number,
		BackupGeneratedAt:  backup.GeneratedAt,
		EventName:          backup.EventName,
		SchemaVersion:      backup.Version,
		Labels:             append([]string(nil), backup.Issue.Labels...),
		Comments:           len(backup.Comments),
		TranscriptMessages: len(backup.Transcript),
		IssueTitleSHA:      shortDocumentHash(backup.Issue.Title),
		IssueBodySHA:       shortDocumentHash(backup.Issue.Body),
		CommentBodySHAs:    make([]string, 0, len(backup.Comments)),
	}
	for _, comment := range backup.Comments {
		plan.CommentBodySHAs = append(plan.CommentBodySHAs, shortDocumentHash(comment.Body))
		if HasGitClawMarker(comment.Body) {
			plan.AssistantTurns++
		}
		if HasGitClawErrorMarker(comment.Body) {
			plan.ErrorComments++
		}
	}
	plan.TranscriptMessageSHAs = make([]string, 0, len(backup.Transcript))
	for _, msg := range backup.Transcript {
		plan.TranscriptMessageSHAs = append(plan.TranscriptMessageSHAs, shortDocumentHash(msg.Body))
		switch msg.Role {
		case "assistant":
			plan.AssistantMessages++
		default:
			plan.UserMessages++
		}
	}
	return plan
}

func writeHashList(b *strings.Builder, prefix string, hashes []string) {
	if len(hashes) == 0 {
		b.WriteString("- none\n")
		return
	}
	for i, hash := range hashes {
		fmt.Fprintf(b, "- %s_%d_sha256_12: `%s`\n", prefix, i+1, hash)
	}
}

func backupJSONLRecord(backup IssueBackup, msg TranscriptMessage, sequence int) BackupJSONLRecord {
	source := "issue"
	if msg.CommentID > 0 {
		source = fmt.Sprintf("comment:%d", msg.CommentID)
	}
	return BackupJSONLRecord{
		Schema:            "gitclaw.backup.transcript.v1",
		Repo:              backup.Repo,
		IssueNumber:       backup.Issue.Number,
		IssueTitle:        backup.Issue.Title,
		BackupGeneratedAt: backup.GeneratedAt,
		EventName:         backup.EventName,
		Sequence:          sequence,
		Source:            source,
		Role:              msg.Role,
		Actor:             msg.Actor,
		AuthorAssociation: msg.AuthorAssociation,
		CommentID:         msg.CommentID,
		Edited:            msg.Edited,
		Trusted:           msg.Trusted,
		BodySHA:           shortDocumentHash(msg.Body),
		Body:              msg.Body,
	}
}

func (r *BackupVerifyResult) verifyIndexIssue(repoDir, repo string, issue BackupIndexIssue, seenNumbers map[int]bool, seenPaths map[string]bool, lastNumber *int) {
	r.IssuesChecked++
	if issue.Number <= 0 {
		r.addFailure("issue_number", fmt.Sprintf("invalid issue number %d", issue.Number))
	}
	if seenNumbers[issue.Number] {
		r.addFailure("issue_unique", fmt.Sprintf("duplicate issue #%d", issue.Number))
	}
	seenNumbers[issue.Number] = true
	if issue.Number < *lastNumber {
		r.addFailure("index_sorted", fmt.Sprintf("issue #%d appears after #%d", issue.Number, *lastNumber))
	}
	*lastNumber = issue.Number

	wantPath := filepath.ToSlash(filepath.Join("issues", fmt.Sprintf("%06d.json", issue.Number)))
	if issue.Path != wantPath {
		r.addFailure("issue_path_canonical", fmt.Sprintf("issue #%d path %q, want %q", issue.Number, issue.Path, wantPath))
	}
	absPath, err := safeBackupPayloadPath(repoDir, issue.Path)
	if err != nil {
		r.addFailure("issue_path_safe", err.Error())
		return
	}
	seenPaths[filepath.ToSlash(issue.Path)] = true
	data, err := os.ReadFile(absPath)
	if err != nil {
		r.addFailure("issue_payload_readable", fmt.Sprintf("%s: %v", issue.Path, err))
		return
	}
	var backup IssueBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		r.addFailure("issue_payload_json", fmt.Sprintf("%s: %v", issue.Path, err))
		return
	}
	r.CommentsChecked += len(backup.Comments)
	r.TranscriptMessages += len(backup.Transcript)
	if backup.Version != 1 {
		r.addFailure("issue_payload_version", fmt.Sprintf("%s version %d, want 1", issue.Path, backup.Version))
	}
	if backup.Repo != repo {
		r.addFailure("issue_payload_repo", fmt.Sprintf("%s repo %q, want %q", issue.Path, backup.Repo, repo))
	}
	if backup.Issue.Number != issue.Number {
		r.addFailure("issue_payload_number", fmt.Sprintf("%s number %d, want %d", issue.Path, backup.Issue.Number, issue.Number))
	}
	if backup.Issue.Title != issue.Title {
		r.addFailure("issue_payload_title", fmt.Sprintf("issue #%d title hash mismatch", issue.Number))
	}
	if len(backup.Comments) != issue.CommentCount {
		r.addFailure("issue_comment_count", fmt.Sprintf("issue #%d got %d, want %d", issue.Number, len(backup.Comments), issue.CommentCount))
	}
	if len(backup.Transcript) != issue.TranscriptMessages {
		r.addFailure("issue_transcript_count", fmt.Sprintf("issue #%d got %d, want %d", issue.Number, len(backup.Transcript), issue.TranscriptMessages))
	}
	if _, err := time.Parse(time.RFC3339, backup.GeneratedAt); err != nil {
		r.addFailure("issue_generated_at", fmt.Sprintf("issue #%d timestamp is not RFC3339", issue.Number))
	}
}

func (r *BackupVerifyResult) verifyNoUnindexedIssues(repoDir string, seenPaths map[string]bool) {
	matches, err := filepath.Glob(filepath.Join(repoDir, "issues", "*.json"))
	if err != nil {
		r.addFailure("issue_glob", err.Error())
		return
	}
	for _, match := range matches {
		rel, err := filepath.Rel(repoDir, match)
		if err != nil {
			r.addFailure("issue_relpath", err.Error())
			continue
		}
		rel = filepath.ToSlash(rel)
		if !seenPaths[rel] {
			r.UnindexedIssueFiles++
			r.addFailure("issue_indexed", fmt.Sprintf("%s is not listed in index.json", rel))
		}
	}
}

func safeBackupPayloadPath(repoDir, relPath string) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		return "", fmt.Errorf("empty backup payload path")
	}
	clean := filepath.Clean(filepath.FromSlash(relPath))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe backup payload path %q", relPath)
	}
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(filepath.Join(absRepoDir, clean))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRepoDir, absPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe backup payload path %q", relPath)
	}
	return absPath, nil
}

func (r *BackupVerifyResult) addFailure(name, detail string) {
	r.VerificationFailures = append(r.VerificationFailures, fmt.Sprintf("%s: %s", name, detail))
}

func readBackupIndexIssue(repoDir, path string) (BackupIndexIssue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BackupIndexIssue{}, err
	}
	var backup IssueBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return BackupIndexIssue{}, err
	}
	rel, err := filepath.Rel(repoDir, path)
	if err != nil {
		return BackupIndexIssue{}, err
	}
	return BackupIndexIssue{
		Number:             backup.Issue.Number,
		Title:              backup.Issue.Title,
		Path:               filepath.ToSlash(rel),
		BackupGeneratedAt:  backup.GeneratedAt,
		EventName:          backup.EventName,
		Labels:             append([]string(nil), backup.Issue.Labels...),
		CommentCount:       len(backup.Comments),
		TranscriptMessages: len(backup.Transcript),
	}, nil
}

func RenderBackupIndexReadme(index BackupIndex) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GitClaw Backups for `%s`\n\n", index.Repo)
	fmt.Fprintf(&b, "- generated_at: `%s`\n", index.GeneratedAt)
	fmt.Fprintf(&b, "- issue_count: `%d`\n\n", index.Count)
	b.WriteString("| Issue | Title | Generated | Comments | Transcript | Path |\n")
	b.WriteString("| --- | --- | --- | ---: | ---: | --- |\n")
	for _, issue := range index.Issues {
		fmt.Fprintf(&b, "| #%d | %s | `%s` | %d | %d | `%s` |\n",
			issue.Number,
			escapeMarkdownTableCell(issue.Title),
			issue.BackupGeneratedAt,
			issue.CommentCount,
			issue.TranscriptMessages,
			issue.Path,
		)
	}
	return b.String()
}

func escapeMarkdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.TrimSpace(value)
}

func backupComments(comments []Comment) []IssueBackupComment {
	out := make([]IssueBackupComment, 0, len(comments))
	for _, comment := range comments {
		out = append(out, IssueBackupComment{
			ID:                comment.ID,
			Body:              comment.Body,
			Author:            comment.User.Login,
			AuthorAssociation: comment.AuthorAssociation,
			CreatedAt:         comment.CreatedAt,
			UpdatedAt:         comment.UpdatedAt,
		})
	}
	return out
}

func issueBackupPath(outDir, repo string, issueNumber int) string {
	return filepath.Join(backupRepoDir(outDir, repo), "issues", fmt.Sprintf("%06d.json", issueNumber))
}

func backupRepoDir(outDir, repo string) string {
	return filepath.Join(outDir, strings.ReplaceAll(repo, "/", "__"))
}
