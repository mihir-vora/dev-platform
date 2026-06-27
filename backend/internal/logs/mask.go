package logs

import (
	"regexp"
	"strings"
)

var (
	secretPattern = regexp.MustCompile(`(?i)(token|secret|password|api[_-]?key)\s*[:=]\s*\S+`)
	githubToken   = regexp.MustCompile(`ghp_[A-Za-z0-9]+`)
	gitlabToken   = regexp.MustCompile(`glpat-[A-Za-z0-9_-]+`)
	bearerToken   = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._-]+`)
)

func Mask(message string) string {
	masked := secretPattern.ReplaceAllString(message, "$1=[REDACTED]")
	masked = githubToken.ReplaceAllString(masked, "ghp_[REDACTED]")
	masked = gitlabToken.ReplaceAllString(masked, "glpat-[REDACTED]")
	masked = bearerToken.ReplaceAllString(masked, "Bearer [REDACTED]")
	return strings.TrimSpace(masked)
}
