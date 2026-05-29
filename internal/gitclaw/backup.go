package gitclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	repoDir := strings.ReplaceAll(repo, "/", "__")
	return filepath.Join(outDir, repoDir, "issues", fmt.Sprintf("%06d.json", issueNumber))
}
