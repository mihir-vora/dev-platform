package logs

import "testing"

func TestMaskSecrets(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"token=abc123", "token=[REDACTED]"},
		{"API_KEY: super-secret", "API_KEY=[REDACTED]"},
		{"Authorization: Bearer eyJhbGciOi", "Authorization: Bearer [REDACTED]"},
		{"using ghp_1234567890abcdef", "using ghp_[REDACTED]"},
		{"plain log line", "plain log line"},
	}
	for _, tc := range cases {
		if got := Mask(tc.in); got != tc.want {
			t.Fatalf("Mask(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
