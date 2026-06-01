package gitclaw

import "testing"

func TestDoctorScriptHasModelFollowupCoverageRequiresRealCommentTurn(t *testing.T) {
	weak := `
echo "GitHub Models"
prompt_context_sha256_12
gitclaw.search_files
`
	if doctorScriptHasModelFollowupCoverage(weak) {
		t.Fatalf("weak model marker text should not count as model follow-up coverage")
	}

	real := `
gh issue comment "$issue_number" --body "Use repo-reader"
model_run_json="$(wait_for_run issue_comment "$comment_started_at")"
wait_for_assistant_count 2
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment"
grep -Fq 'gitclaw.search_files' <<<"$model_comment"
`
	if !doctorScriptHasModelFollowupCoverage(real) {
		t.Fatalf("real issue_comment follow-up script should count as model follow-up coverage")
	}

	heartbeatStyle := `
gh issue comment "$issue_number" --body "@gitclaw continue"
wait_for_issue_comment_run "$comment_started_at"
wait_for_assistant_count 1
grep -Fq 'prompt_context_sha256_12="' <<<"$assistant_comment"
grep -Fq 'gitclaw.search_files' <<<"$assistant_comment"
`
	if !doctorScriptHasModelFollowupCoverage(heartbeatStyle) {
		t.Fatalf("heartbeat-style issue_comment follow-up should count even when it is the first assistant-turn marker")
	}

	issueSpecific := `
gh issue comment "$child_issue_number" --body "@gitclaw continue"
wait_for_issue_comment_run_for_title "$comment_started_at" "$child_issue_title"
wait_for_assistant_count_for_issue "$child_issue_number" 1
grep -Fq 'prompt_context_sha256_12="' <<<"$assistant_comment"
grep -Fq 'gitclaw.search_files' <<<"$assistant_comment"
`
	if !doctorScriptHasModelFollowupCoverage(issueSpecific) {
		t.Fatalf("issue-specific assistant count helpers should count as model follow-up coverage")
	}
}
