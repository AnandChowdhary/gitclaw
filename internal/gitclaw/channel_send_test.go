package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRunChannelSendQueuesOutboundMessage(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{}

	result, err := RunChannelSend(context.Background(), cfg, github, ChannelSendOptions{
		Repo:      "owner/repo",
		Channel:   "Telegram",
		ThreadID:  "chat-123",
		MessageID: "notify-456",
		Author:    "gitclaw:proactive",
		Body:      "Outbound channel body with CHANNEL_SEND_TOKEN.",
	})
	if err != nil {
		t.Fatalf("RunChannelSend returned error: %v", err)
	}
	if result.IssueNumber == 0 || result.CommentID == 0 || !result.Created || result.Duplicate {
		t.Fatalf("unexpected send result: %#v", result)
	}
	issue := github.Issues[0]
	if !HasChannelThreadMarker(issue.Body) || !strings.Contains(issue.Body, `thread_id="chat-123"`) {
		t.Fatalf("created issue missing channel thread marker: %#v", issue)
	}
	if hasLabel(github.IssueLabels[result.IssueNumber], cfg.TriggerLabel) {
		t.Fatalf("channel send should not apply the model trigger label: %#v", github.IssueLabels[result.IssueNumber])
	}
	if !hasLabel(github.IssueLabels[result.IssueNumber], cfg.ChannelLabel) {
		t.Fatalf("channel issue missing channel label: %#v", github.IssueLabels[result.IssueNumber])
	}
	comments := github.CommentsByIssue[result.IssueNumber]
	if len(comments) != 1 {
		t.Fatalf("comments = %d, want 1: %#v", len(comments), comments)
	}
	body := comments[0].Body
	for _, want := range []string{`gitclaw:channel-outbound`, `channel="telegram"`, `thread_id="chat-123"`, `message_id="notify-456"`, "CHANNEL_SEND_TOKEN"} {
		if !strings.Contains(body, want) {
			t.Fatalf("outbound comment missing %q:\n%s", want, body)
		}
	}
}

func TestRunChannelSendResolvesNamedRoute(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workdir = t.TempDir()
	writeTestFile(t, cfg.Workdir, channelRoutesPath, `routes:
  - name: team-demo
    channel: slack
    thread_id_template: route-thread-{message_id}
    author: gitclaw:route
`)
	github := &FakeGitHub{}

	result, err := RunChannelSend(context.Background(), cfg, github, ChannelSendOptions{
		Repo:      "owner/repo",
		Route:     "TEAM-demo",
		MessageID: "notify-789",
		Body:      "Route body with CHANNEL_ROUTE_TOKEN.",
	})
	if err != nil {
		t.Fatalf("RunChannelSend returned error: %v", err)
	}
	if !result.Created || result.RouteName != "team-demo" || result.RouteHash == "" {
		t.Fatalf("unexpected routed send result: %#v", result)
	}
	issue := github.Issues[0]
	if !strings.Contains(issue.Body, `channel="slack"`) || !strings.Contains(issue.Body, `thread_id="route-thread-notify-789"`) {
		t.Fatalf("created issue missing routed channel marker: %s", issue.Body)
	}
	comment := github.CommentsByIssue[result.IssueNumber][0].Body
	for _, want := range []string{`author="gitclaw:route"`, `thread_id="route-thread-notify-789"`, "CHANNEL_ROUTE_TOKEN"} {
		if !strings.Contains(comment, want) {
			t.Fatalf("routed outbound comment missing %q:\n%s", want, comment)
		}
	}
}

func TestRunChannelSendRejectsConflictingNamedRoute(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workdir = t.TempDir()
	writeTestFile(t, cfg.Workdir, channelRoutesPath, `routes:
  - name: team-demo
    channel: slack
    thread_id: slack-thread
`)

	_, err := RunChannelSend(context.Background(), cfg, &FakeGitHub{}, ChannelSendOptions{
		Repo:      "owner/repo",
		Route:     "team-demo",
		Channel:   "telegram",
		MessageID: "notify-789",
		Body:      "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "resolves channel") {
		t.Fatalf("expected channel conflict error, got %v", err)
	}
}

func TestRunChannelSendReusesThreadAndDedupesMessage(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 7,
			Title:  "GitClaw slack thread channel-123",
			Body: RenderChannelThreadBody(ChannelIngestOptions{
				Channel:  "slack",
				ThreadID: "channel-123",
			}),
			Labels: []string{cfg.ChannelLabel},
		}},
		CommentsByIssue: map[int][]Comment{
			7: {{
				ID: 99,
				Body: RenderChannelOutboundComment(ChannelSendOptions{
					Channel:   "slack",
					ThreadID:  "channel-123",
					MessageID: "notify-456",
					Body:      "already queued",
				}),
			}},
		},
	}
	result, err := RunChannelSend(context.Background(), cfg, github, ChannelSendOptions{
		Repo:      "owner/repo",
		Channel:   "slack",
		ThreadID:  "channel-123",
		MessageID: "notify-456",
		Body:      "already queued",
	})
	if err != nil {
		t.Fatalf("RunChannelSend returned error: %v", err)
	}
	if result.IssueNumber != 7 || result.Created || !result.Duplicate {
		t.Fatalf("unexpected duplicate send result: %#v", result)
	}
	if len(github.CommentsByIssue[7]) != 1 {
		t.Fatalf("duplicate send posted another comment: %#v", github.CommentsByIssue[7])
	}
}
