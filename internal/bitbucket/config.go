package bitbucket

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
)

type Config struct {
	BaseURL    *url.URL
	ProjectKey string
	Token      string
	UserSlug   string
	CAFile     string
}

func LoadConfig(getenv func(string) string, shared config.Shared) (Config, bool, error) {
	rawBase := strings.TrimSpace(getenv("BITBUCKET_BASE_URL"))
	project := strings.TrimSpace(getenv("BITBUCKET_PROJECT_KEY"))
	token := getenv("BITBUCKET_BEARER_TOKEN")
	userSlug := strings.TrimSpace(getenv("BITBUCKET_USER_SLUG"))
	ca := strings.TrimSpace(getenv("BITBUCKET_CA_FILE"))
	if rawBase == "" && project == "" && token == "" && userSlug == "" && ca == "" {
		return Config{}, false, nil
	}
	if rawBase == "" || project == "" || token == "" {
		return Config{}, true, fmt.Errorf("BITBUCKET_BASE_URL, BITBUCKET_PROJECT_KEY, and BITBUCKET_BEARER_TOKEN are required")
	}
	u, err := url.Parse(rawBase)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Config{}, true, fmt.Errorf("BITBUCKET_BASE_URL must be an http or https URL")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return Config{}, true, fmt.Errorf("BITBUCKET_BASE_URL must not include query or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if shared.TLSVerify && ca != "" {
		if _, err := os.Stat(ca); err != nil {
			return Config{}, true, fmt.Errorf("BITBUCKET_CA_FILE is not readable")
		}
	}
	return Config{BaseURL: u, ProjectKey: project, Token: token, UserSlug: userSlug, CAFile: ca}, true, nil
}
