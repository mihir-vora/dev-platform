package validate

import (
	"fmt"
	"net/url"
	"strings"
)

func RepoURL(repoURL, gitProvider, gitlabBaseURL string) error {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("invalid repository URL")
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("repository URL must use https")
	}
	if parsed.User != nil {
		return fmt.Errorf("repository URL must not contain credentials")
	}

	host := strings.ToLower(parsed.Hostname())
	path := strings.Trim(parsed.Path, "/")
	if path == "" || strings.Count(path, "/") < 1 {
		return fmt.Errorf("repository URL must include owner and repository name")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("invalid repository path")
	}

	switch gitProvider {
	case "github":
		if host != "github.com" && host != "www.github.com" {
			return fmt.Errorf("repository URL host must be github.com for GitHub projects")
		}
	case "gitlab":
		gitlabHost := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(gitlabBaseURL, "https://"), "http://"))
		gitlabHost = strings.TrimSuffix(gitlabHost, "/")
		if host != gitlabHost && host != "gitlab.com" && host != "www.gitlab.com" {
			return fmt.Errorf("repository URL host must match configured GitLab host")
		}
	default:
		return fmt.Errorf("unsupported git provider")
	}

	return nil
}

func OneOf(value string, allowed ...string) error {
	for _, item := range allowed {
		if value == item {
			return nil
		}
	}
	return fmt.Errorf("invalid value %q", value)
}
