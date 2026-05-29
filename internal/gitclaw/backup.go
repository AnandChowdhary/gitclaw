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
