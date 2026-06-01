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

func (c *RESTGitHubClient) CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error) {
	if c.Token == "" {
		return Issue{}, fmt.Errorf("missing GitHub token")
	}
	payload := map[string]any{
		"title": title,
		"body":  body,
	}
	if len(labels) > 0 {
		payload["labels"] = labels
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Issue{}, err
	}
	endpoint := fmt.Sprintf("%s/repos/%s/issues", strings.TrimRight(c.APIBaseURL, "/"), repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return Issue{}, err
	}
	c.setHeaders(req)
	res, err := c.httpClient().Do(req)
	if err != nil {
		return Issue{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return Issue{}, fmt.Errorf("GitHub create issue failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw struct {
		Number            int    `json:"number"`
		Title             string `json:"title"`
		Body              string `json:"body"`
		AuthorAssociation string `json:"author_association"`
		User              User   `json:"user"`
		Labels            []struct {
			Name string `json:"name"`
		} `json:"labels"`
		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return Issue{}, err
	}
	return issueFromREST(raw.Number, raw.Title, raw.Body, raw.AuthorAssociation, raw.User, raw.Labels, raw.PullRequest != nil), nil
}

func (c *RESTGitHubClient) GetIssue(ctx context.Context, repo string, issueNumber int) (Issue, error) {
	if c.Token == "" {
		return Issue{}, fmt.Errorf("missing GitHub token")
	}
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d", strings.TrimRight(c.APIBaseURL, "/"), repo, issueNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Issue{}, err
	}
	c.setHeaders(req)
	res, err := c.httpClient().Do(req)
	if err != nil {
		return Issue{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return Issue{}, fmt.Errorf("GitHub get issue failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw struct {
		Number            int    `json:"number"`
		Title             string `json:"title"`
		Body              string `json:"body"`
		AuthorAssociation string `json:"author_association"`
		User              User   `json:"user"`
		Labels            []struct {
			Name string `json:"name"`
		} `json:"labels"`
		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return Issue{}, err
	}
	return issueFromREST(raw.Number, raw.Title, raw.Body, raw.AuthorAssociation, raw.User, raw.Labels, raw.PullRequest != nil), nil
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

func (c *RESTGitHubClient) ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("missing GitHub token")
	}
	if limit <= 0 {
		limit = 20
	}
	issues := make([]Issue, 0, limit)
	page := 1
	for len(issues) < limit {
		perPage := limit - len(issues)
		if perPage > 100 {
			perPage = 100
		}
		values := url.Values{}
		values.Set("state", "open")
		values.Set("sort", "created")
		values.Set("direction", "desc")
		values.Set("per_page", fmt.Sprintf("%d", perPage))
		values.Set("page", fmt.Sprintf("%d", page))
		if len(labels) > 0 {
			values.Set("labels", strings.Join(labels, ","))
		}
		endpoint := fmt.Sprintf("%s/repos/%s/issues?%s", strings.TrimRight(c.APIBaseURL, "/"), repo, values.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		c.setHeaders(req)
		res, err := c.httpClient().Do(req)
		if err != nil {
			return nil, err
		}
		var raw []struct {
			Number            int    `json:"number"`
			Title             string `json:"title"`
			Body              string `json:"body"`
			AuthorAssociation string `json:"author_association"`
			User              User   `json:"user"`
			Labels            []struct {
				Name string `json:"name"`
			} `json:"labels"`
			PullRequest *struct {
				URL string `json:"url"`
			} `json:"pull_request"`
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
			res.Body.Close()
			return nil, fmt.Errorf("GitHub list issues failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
		}
		if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
			res.Body.Close()
			return nil, err
		}
		res.Body.Close()
		for _, item := range raw {
			issues = append(issues, issueFromREST(item.Number, item.Title, item.Body, item.AuthorAssociation, item.User, item.Labels, item.PullRequest != nil))
		}
		if len(raw) < perPage {
			break
		}
		page++
	}
	return issues, nil
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

func (c *RESTGitHubClient) CloseIssue(ctx context.Context, repo string, issueNumber int) error {
	if c.Token == "" {
		return fmt.Errorf("missing GitHub token")
	}
	payload, err := json.Marshal(map[string]string{"state": "closed"})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d", strings.TrimRight(c.APIBaseURL, "/"), repo, issueNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	res, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("GitHub close issue failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *RESTGitHubClient) AddIssueLabels(ctx context.Context, repo string, issueNumber int, labels []string) error {
	if c.Token == "" {
		return fmt.Errorf("missing GitHub token")
	}
	if len(labels) == 0 {
		return nil
	}
	payload, err := json.Marshal(map[string][]string{"labels": labels})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d/labels", strings.TrimRight(c.APIBaseURL, "/"), repo, issueNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	res, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("GitHub add labels failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *RESTGitHubClient) RemoveIssueLabel(ctx context.Context, repo string, issueNumber int, label string) error {
	if c.Token == "" {
		return fmt.Errorf("missing GitHub token")
	}
	if label == "" {
		return nil
	}
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d/labels/%s", strings.TrimRight(c.APIBaseURL, "/"), repo, issueNumber, url.PathEscape(label))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)
	res, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("GitHub remove label failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
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

func issueFromREST(number int, title, body, association string, user User, rawLabels []struct {
	Name string `json:"name"`
}, isPullRequest bool) Issue {
	labels := make([]string, 0, len(rawLabels))
	for _, label := range rawLabels {
		labels = append(labels, label.Name)
	}
	return Issue{
		Number:            number,
		Title:             title,
		Body:              body,
		AuthorAssociation: association,
		User:              user,
		Labels:            labels,
		IsPullRequest:     isPullRequest,
	}
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
