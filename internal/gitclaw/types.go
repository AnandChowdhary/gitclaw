package gitclaw

import (
	"context"
)

const (
	EventIssueOpened  = "issue_opened"
	EventIssueComment = "issue_comment"
)

type Config struct {
	TriggerLabel        string
	TriggerPrefix       string
	DisabledLabel       string
	AllowedAssociations map[string]bool
	Model               string
	Workdir             string
}

func DefaultConfig() Config {
	return Config{
		TriggerLabel:  "gitclaw",
		TriggerPrefix: "@gitclaw",
		DisabledLabel: "gitclaw:disabled",
		AllowedAssociations: map[string]bool{
			"OWNER":        true,
			"MEMBER":       true,
			"COLLABORATOR": true,
		},
		Model:   "openai/gpt-5-mini",
		Workdir: ".",
	}
}

type Event struct {
	Kind      string
	EventName string
	Action    string
	Repo      string
	SHA       string
	Issue     Issue
	Comment   *Comment
	Sender    User
}

type Issue struct {
	Number            int
	Title             string
	Body              string
	AuthorAssociation string
	User              User
	Labels            []string
	IsPullRequest     bool
}

type Comment struct {
	ID                int64
	Body              string
	AuthorAssociation string
	User              User
	CreatedAt         string
	UpdatedAt         string
}

type User struct {
	Login string
	Type  string
}

func (u User) IsBot() bool {
	return u.Type == "Bot" || u.Login == "github-actions[bot]"
}

type PreflightDecision struct {
	Allowed bool
	Code    string
	Reason  string
}

type TranscriptMessage struct {
	Role              string
	Body              string
	Actor             string
	AuthorAssociation string
	CommentID         int64
	Edited            bool
	Trusted           bool
}

type Marker struct {
	RunID          string
	EventID        string
	Model          string
	IdempotencyKey string
	RunURL         string
}

type PostedComment struct {
	ID   int64
	Body string
	URL  string
}

type GitHubClient interface {
	ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error)
	PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error)
}

type LLMClient interface {
	Complete(ctx context.Context, req LLMRequest) (string, error)
}

type LLMRequest struct {
	Event      Event
	Transcript []TranscriptMessage
	Context    RepoContext
	Config     Config
}

type RepoContext struct {
	Documents   []ContextDocument
	Skills      []ContextDocument
	ToolOutputs []ToolOutput
}

type ContextDocument struct {
	Path string
	Body string
}

type SkillSummary struct {
	Name        string
	Description string
	Path        string
	Always      bool
}

type ToolOutput struct {
	Name   string
	Input  string
	Output string
}
