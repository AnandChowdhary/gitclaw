package gitclaw

import (
	"context"
)

const (
	EventIssueOpened      = "issue_opened"
	EventIssueComment     = "issue_comment"
	EventWorkflowDispatch = "workflow_dispatch"
)

type Config struct {
	TriggerLabel              string
	TriggerPrefix             string
	DisabledLabel             string
	RunningLabel              string
	DoneLabel                 string
	ErrorLabel                string
	HeartbeatLabel            string
	ChannelLabel              string
	ProactiveLabel            string
	WriteRequestedLabel       string
	AllowedAssociations       map[string]bool
	ModelProvider             string
	Model                     string
	ModelFallbacks            []string
	LLMBaseURL                string
	Workdir                   string
	ConfigSource              string
	MaxPromptBytes            int
	MaxOutputTokens           int
	MaxTranscriptMessages     int
	MaxTranscriptMessageBytes int
	AllowedSkills             map[string]bool
	DisabledSkills            map[string]bool
	AllowedTools              map[string]bool
	DisabledTools             map[string]bool
}

func DefaultConfig() Config {
	return Config{
		TriggerLabel:        "gitclaw",
		TriggerPrefix:       "@gitclaw",
		DisabledLabel:       "gitclaw:disabled",
		RunningLabel:        "gitclaw:running",
		DoneLabel:           "gitclaw:done",
		ErrorLabel:          "gitclaw:error",
		HeartbeatLabel:      "gitclaw:heartbeat",
		ChannelLabel:        "gitclaw:channel",
		ProactiveLabel:      "gitclaw:proactive",
		WriteRequestedLabel: "gitclaw:write-requested",
		AllowedAssociations: map[string]bool{
			"OWNER":        true,
			"MEMBER":       true,
			"COLLABORATOR": true,
		},
		ModelProvider:             "github-models",
		Model:                     "openai/gpt-5-nano",
		LLMBaseURL:                defaultGitHubModelsBaseURL,
		Workdir:                   ".",
		ConfigSource:              "defaults",
		MaxPromptBytes:            60000,
		MaxOutputTokens:           4000,
		MaxTranscriptMessages:     40,
		MaxTranscriptMessageBytes: 8000,
	}
}

type Event struct {
	Kind           string
	EventName      string
	Action         string
	Repo           string
	SHA            string
	Issue          Issue
	Comment        *Comment
	Sender         User
	DispatchID     string
	DispatchReason string
	ActiveText     string
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

type HeartbeatMarker struct {
	RunID  string
	Slot   string
	RunURL string
}

type ErrorMarker struct {
	RunID   string
	EventID string
	Phase   string
	RunURL  string
}

type PostedComment struct {
	ID   int64
	Body string
	URL  string
}

type GitHubClient interface {
	GetIssue(ctx context.Context, repo string, issueNumber int) (Issue, error)
	ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error)
	PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error)
	AddIssueLabels(ctx context.Context, repo string, issueNumber int, labels []string) error
	RemoveIssueLabel(ctx context.Context, repo string, issueNumber int, label string) error
}

type ChannelIngestGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
	ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error)
	PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error)
	AddIssueLabels(ctx context.Context, repo string, issueNumber int, labels []string) error
}

type ChannelStateGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
	ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error)
	PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error)
	AddIssueLabels(ctx context.Context, repo string, issueNumber int, labels []string) error
}

type ChannelGatewayGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
	ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error)
	PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error)
	AddIssueLabels(ctx context.Context, repo string, issueNumber int, labels []string) error
}

type ChannelDeliveryGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
	ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error)
	PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error)
	AddIssueLabels(ctx context.Context, repo string, issueNumber int, labels []string) error
}

type ProactiveGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
	AddIssueLabels(ctx context.Context, repo string, issueNumber int, labels []string) error
}

type HeartbeatGitHubClient interface {
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
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
	Documents         []ContextDocument
	ContextReferences []ContextReferenceSummary
	Skills            []ContextDocument
	SkillSummaries    []SkillSummary
	SkillBundles      []SkillBundleSummary
	ToolOutputs       []ToolOutput
	AllowedTools      map[string]bool
	DisabledTools     map[string]bool
}

type ContextDocument struct {
	Path string
	Body string
}

type ContextReferenceSummary struct {
	Kind      string
	Path      string
	LineRange string
	Count     int
	Status    string
	Reason    string
	Bytes     int
	Lines     int
	Entries   int
	SHA       string
}

type SkillSummary struct {
	Name               string
	Description        string
	Path               string
	Always             bool
	Enabled            bool
	DisabledByConfig   bool
	BlockedByAllowlist bool
	FrontmatterPresent bool
	Bytes              int
	Lines              int
	SHA                string
	RequiredEnv        []string
	RequiredBins       []string
	MissingEnv         []string
	MissingBins        []string
}

type SkillBundleSummary struct {
	Name               string
	Description        string
	Path               string
	Skills             []string
	ResolvedSkills     []string
	MissingSkills      []string
	Selected           bool
	InstructionPresent bool
	Bytes              int
	Lines              int
	SHA                string
}

type ToolOutput struct {
	Name   string
	Input  string
	Output string
}
