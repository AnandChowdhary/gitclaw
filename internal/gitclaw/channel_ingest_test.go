package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestChannelIngestCreatesThreadIssueAndMirrorsMessage(t *testing.T) {
	github := &FakeGitHub{}
	result, err := RunChannelIngest(context.Background(), DefaultConfig(), github, ChannelIngestOptions{
		Repo:      "owner/repo",
		Channel:   "telegram",
		ThreadID:  "chat-123",
		MessageID: "update-456",
		Author:    "telegram:alice",
		Body:      "Please include CHANNEL_INGEST_TOKEN.",
	})
	if err != nil {
		t.Fatalf("RunChannelIngest returned error: %v", err)
	}
	if result.IssueNumber == 0 || !result.Created || result.Duplicate {
		t.Fatalf("unexpected ingest result: %#v", result)
	}
	issue := github.Issues[0]
	if !HasChannelThreadMarker(issue.Body) || !strings.Contains(issue.Body, `thread_id="chat-123"`) {
		t.Fatalf("created issue missing channel thread marker: %#v", issue)
	}
	if !hasLabel(github.IssueLabels[result.IssueNumber], "gitclaw") || !hasLabel(github.IssueLabels[result.IssueNumber], "gitclaw:channel") {
		t.Fatalf("channel issue labels missing: %#v", github.IssueLabels[result.IssueNumber])
	}
	comments := github.CommentsByIssue[result.IssueNumber]
	if len(comments) != 1 {
		t.Fatalf("comments = %d, want 1: %#v", len(comments), comments)
	}
	if !HasChannelMessageMarker(comments[0].Body) || !strings.Contains(comments[0].Body, "CHANNEL_INGEST_TOKEN") {
		t.Fatalf("channel message comment missing marker or body: %s", comments[0].Body)
	}
}

func TestChannelIngestReusesThreadIssueAndDedupesMessage(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 7,
			Title:  "GitClaw telegram thread chat-123",
			Body: RenderChannelThreadBody(ChannelIngestOptions{
				Channel:  "telegram",
				ThreadID: "chat-123",
			}),
			Labels: []string{cfg.ChannelLabel},
		}},
		CommentsByIssue: map[int][]Comment{
			7: {{
				ID: 99,
				Body: RenderChannelMessageComment(ChannelIngestOptions{
					Channel:   "telegram",
					ThreadID:  "chat-123",
					MessageID: "update-456",
					Body:      "already mirrored",
				}),
			}},
		},
	}
	result, err := RunChannelIngest(context.Background(), cfg, github, ChannelIngestOptions{
		Repo:      "owner/repo",
		Channel:   "telegram",
		ThreadID:  "chat-123",
		MessageID: "update-456",
		Body:      "already mirrored",
	})
	if err != nil {
		t.Fatalf("RunChannelIngest returned error: %v", err)
	}
	if result.IssueNumber != 7 || result.Created || !result.Duplicate {
		t.Fatalf("unexpected ingest result: %#v", result)
	}
	if len(github.CommentsByIssue[7]) != 1 {
		t.Fatalf("duplicate ingest posted another comment: %#v", github.CommentsByIssue[7])
	}
}
