package gitclaw

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func ParseEvent(eventName string, payload []byte) (Event, error) {
	var raw struct {
		Action     string `json:"action"`
		After      string `json:"after"`
		Repository struct {
			FullName      string `json:"full_name"`
			DefaultBranch string `json:"default_branch"`
		} `json:"repository"`
		Issue struct {
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
		} `json:"issue"`
		Comment *struct {
			ID                int64  `json:"id"`
			Body              string `json:"body"`
			AuthorAssociation string `json:"author_association"`
			User              User   `json:"user"`
			CreatedAt         string `json:"created_at"`
			UpdatedAt         string `json:"updated_at"`
		} `json:"comment"`
		Inputs map[string]string `json:"inputs"`
		Sender User              `json:"sender"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return Event{}, fmt.Errorf("parse event JSON: %w", err)
	}

	labels := make([]string, 0, len(raw.Issue.Labels))
	for _, label := range raw.Issue.Labels {
		labels = append(labels, label.Name)
	}

	ev := Event{
		EventName: eventName,
		Action:    raw.Action,
		Repo:      raw.Repository.FullName,
		SHA:       raw.After,
		Issue: Issue{
			Number:            raw.Issue.Number,
			Title:             raw.Issue.Title,
			Body:              raw.Issue.Body,
			AuthorAssociation: raw.Issue.AuthorAssociation,
			User:              raw.Issue.User,
			Labels:            labels,
			IsPullRequest:     raw.Issue.PullRequest != nil,
		},
		Sender: raw.Sender,
	}

	switch eventName {
	case "issues":
		if raw.Action != "opened" {
			return Event{}, fmt.Errorf("unsupported issues action %q", raw.Action)
		}
		ev.Kind = EventIssueOpened
	case "issue_comment":
		if raw.Action != "created" {
			return Event{}, fmt.Errorf("unsupported issue_comment action %q", raw.Action)
		}
		if raw.Comment == nil {
			return Event{}, fmt.Errorf("issue_comment event missing comment")
		}
		ev.Kind = EventIssueComment
		ev.Comment = &Comment{
			ID:                raw.Comment.ID,
			Body:              raw.Comment.Body,
			AuthorAssociation: raw.Comment.AuthorAssociation,
			User:              raw.Comment.User,
			CreatedAt:         raw.Comment.CreatedAt,
			UpdatedAt:         raw.Comment.UpdatedAt,
		}
	case "workflow_dispatch":
		issueNumber, err := dispatchIssueNumber(raw.Inputs)
		if err != nil {
			return Event{}, err
		}
		ev.Kind = EventWorkflowDispatch
		ev.Action = "requested"
		ev.Issue.Number = issueNumber
		ev.DispatchID = strings.TrimSpace(raw.Inputs["dispatch_id"])
		ev.DispatchReason = strings.TrimSpace(raw.Inputs["reason"])
	default:
		return Event{}, fmt.Errorf("unsupported event name %q", eventName)
	}

	if ev.Repo == "" {
		return Event{}, fmt.Errorf("event missing repository.full_name")
	}
	if ev.Issue.Number == 0 {
		return Event{}, fmt.Errorf("event missing issue.number")
	}
	return ev, nil
}

func dispatchIssueNumber(inputs map[string]string) (int, error) {
	raw := strings.TrimSpace(inputs["issue_number"])
	if raw == "" {
		return 0, fmt.Errorf("workflow_dispatch missing inputs.issue_number")
	}
	issueNumber, err := strconv.Atoi(raw)
	if err != nil || issueNumber <= 0 {
		return 0, fmt.Errorf("invalid workflow_dispatch inputs.issue_number %q", raw)
	}
	return issueNumber, nil
}
