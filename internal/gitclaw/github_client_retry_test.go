package gitclaw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRESTGitHubClientRetriesTransientListIssuesFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/repos/owner/repo/issues" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "temporary GitHub outage", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"number":7,"title":"heartbeat","body":"","user":{"login":"alice","type":"User"},"labels":[{"name":"gitclaw:heartbeat"}]}]`))
	}))
	defer server.Close()

	client := &RESTGitHubClient{
		Token:      "test-token",
		APIBaseURL: server.URL,
		Client:     server.Client(),
	}
	issues, err := client.ListOpenIssues(context.Background(), "owner/repo", []string{"gitclaw:heartbeat"}, 1)
	if err != nil {
		t.Fatalf("ListOpenIssues returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("server calls = %d, want 2", calls)
	}
	if len(issues) != 1 || issues[0].Number != 7 {
		t.Fatalf("issues = %#v, want issue #7", issues)
	}
}

func TestRESTGitHubClientDoesNotRetryPermanentListIssuesFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "repository not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := &RESTGitHubClient{
		Token:      "test-token",
		APIBaseURL: server.URL,
		Client:     server.Client(),
	}
	_, err := client.ListOpenIssues(context.Background(), "owner/repo", nil, 1)
	if err == nil || !strings.Contains(err.Error(), "status=404") {
		t.Fatalf("ListOpenIssues error = %v, want status=404", err)
	}
	if calls != 1 {
		t.Fatalf("server calls = %d, want 1", calls)
	}
}
