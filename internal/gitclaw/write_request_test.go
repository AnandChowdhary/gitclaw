package gitclaw

import "testing"

func TestDetectWriteRequest(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{name: "implement", body: "Please implement a new CLI command.", want: true},
		{name: "open pr", body: "Can you open a PR for this fix?", want: true},
		{name: "explain", body: "Can you explain what would need to change?", want: false},
		{name: "read", body: "Please inspect go.mod and summarize it.", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectWriteRequest([]TranscriptMessage{{Role: "user", Body: tc.body}})
			if got != tc.want {
				t.Fatalf("DetectWriteRequest(%q) = %t, want %t", tc.body, got, tc.want)
			}
		})
	}
}
