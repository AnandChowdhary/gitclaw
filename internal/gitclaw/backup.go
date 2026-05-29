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
