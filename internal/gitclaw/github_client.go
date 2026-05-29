package gitclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type RESTGitHubClient struct {
	Token      string
	APIBaseURL string
	Client     *http.Client
}

func NewRESTGitHubClient(token string) *RESTGitHubClient {
	return &RESTGitHubClient{
		Token:      token,
		APIBaseURL: "https://api.github.com",
		Client:     http.DefaultClient,
	}
}

func (c *RESTGitHubClient) ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("missing GitHub token")
	}
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=100", strings.TrimRight(c.APIBaseURL, "/"), repo, issueNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, fmt.Errorf("GitHub list comments failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw []struct {
		ID                int64  `json:"id"`
		Body              string `json:"body"`
		AuthorAssociation string `json:"author_association"`
		User              User   `json:"user"`
		CreatedAt         string `json:"created_at"`
		UpdatedAt         string `json:"updated_at"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}
	comments := make([]Comment, 0, len(raw))
	for _, item := range raw {
		comments = append(comments, Comment{
			ID:                item.ID,
			Body:              item.Body,
			AuthorAssociation: item.AuthorAssociation,
			User:              item.User,
			CreatedAt:         item.CreatedAt,
			UpdatedAt:         item.UpdatedAt,
		})
	}
	return comments, nil
}

func (c *RESTGitHubClient) PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error) {
	if c.Token == "" {
		return PostedComment{}, fmt.Errorf("missing GitHub token")
	}
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return PostedComment{}, err
	}
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d/comments", strings.TrimRight(c.APIBaseURL, "/"), repo, issueNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return PostedComment{}, err
	}
	c.setHeaders(req)
	res, err := c.httpClient().Do(req)
	if err != nil {
		return PostedComment{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return PostedComment{}, fmt.Errorf("GitHub post comment failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw struct {
		ID      int64  `json:"id"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return PostedComment{}, err
	}
	return PostedComment{ID: raw.ID, Body: raw.Body, URL: raw.HTMLURL}, nil
}

func (c *RESTGitHubClient) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
}

func (c *RESTGitHubClient) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func validateRepoName(repo string) error {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid repository full name %q", repo)
	}
	if _, err := url.PathUnescape(repo); err != nil {
		return fmt.Errorf("invalid repository full name %q: %w", repo, err)
	}
	return nil
}
