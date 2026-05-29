package gitclaw

import "strings"

func Preflight(ev Event, cfg Config) PreflightDecision {
	if ev.Kind == "" {
		return reject("unsupported_event", "unsupported GitHub event")
	}
	if hasLabel(ev.Issue.Labels, cfg.DisabledLabel) {
		return reject("issue_disabled", "issue has disabled label")
	}
	if ev.Kind == EventIssueComment && ev.Issue.IsPullRequest {
		return reject("pr_comment_ignored", "pull request comments are ignored")
	}
	if ev.Sender.IsBot() || (ev.Comment != nil && ev.Comment.User.IsBot()) {
		return reject("bot_comment_ignored", "bot comments are ignored")
	}
	if !triggered(ev, cfg) {
		return reject("not_triggered", "issue is not labeled or prefixed for GitClaw")
	}
	if !trustedAssociation(actorAssociation(ev), cfg) {
		return reject("actor_not_trusted", "actor association is not trusted")
	}
	return PreflightDecision{Allowed: true, Code: "allowed", Reason: "allowed"}
}

func reject(code, reason string) PreflightDecision {
	return PreflightDecision{Allowed: false, Code: code, Reason: reason}
}

func actorAssociation(ev Event) string {
	if ev.Comment != nil {
		return ev.Comment.AuthorAssociation
	}
	return ev.Issue.AuthorAssociation
}

func trustedAssociation(association string, cfg Config) bool {
	return cfg.AllowedAssociations[strings.ToUpper(association)]
}

func triggered(ev Event, cfg Config) bool {
	prefix := cfg.TriggerPrefix
	return hasLabel(ev.Issue.Labels, cfg.TriggerLabel) ||
		strings.HasPrefix(strings.TrimSpace(ev.Issue.Title), prefix) ||
		strings.HasPrefix(strings.TrimSpace(ev.Issue.Body), prefix) ||
		(ev.Comment != nil && strings.HasPrefix(strings.TrimSpace(ev.Comment.Body), prefix))
}

func hasLabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}
