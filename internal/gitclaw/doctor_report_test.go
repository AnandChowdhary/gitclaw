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
}
