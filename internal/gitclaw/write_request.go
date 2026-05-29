package gitclaw

import "strings"

func DetectWriteRequest(transcript []TranscriptMessage) bool {
	text := strings.ToLower(transcriptText(transcript))
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return false
	}
	phrases := []string{
		"apply patch",
		"change the code",
		"commit this",
		"create a branch",
		"create a file",
		"delete the file",
		"edit the file",
		"fix this bug",
		"implement this",
		"make changes",
		"modify the file",
		"open a pr",
		"open a pull request",
		"push a branch",
		"refactor this",
		"update the code",
		"write code",
	}
	for _, phrase := range phrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	for _, prefix := range []string{
		"add ",
		"build ",
		"create ",
		"delete ",
		"fix ",
		"implement ",
		"modify ",
		"remove ",
		"rename ",
		"update ",
	} {
		if strings.Contains(text, prefix+"a ") ||
			strings.Contains(text, prefix+"an ") ||
			strings.Contains(text, prefix+"the ") ||
			strings.Contains(text, prefix+"this ") ||
			strings.Contains(text, prefix+"new ") {
			return true
		}
	}
	return false
}

func WriteRequestPolicyOutput() ToolOutput {
	return ToolOutput{
		Name:   "gitclaw.policy",
		Input:  "write-request",
		Output: "Write request detected. Current GitClaw mode is read-only: do not modify files, create branches, commit, push, or open pull requests. Answer by explaining constraints and, when useful, propose a patch or implementation plan for a maintainer to apply.",
	}
}
