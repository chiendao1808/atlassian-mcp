package confluence

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
)

// Config contains static Confluence settings that are safe to validate at startup.
type Config struct {
	BaseURL *url.URL
	CAFile  string
}

// LoadConfig parses Confluence static configuration without making network requests.
func LoadConfig(getenv func(string) string, shared config.Shared) (Config, bool, error) {
	rawBase := strings.TrimSpace(getenv("CONFLUENCE_BASE_URL"))
	if rawBase == "" {
		return Config{}, false, nil
	}
	u, err := url.Parse(rawBase)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Config{}, true, fmt.Errorf("CONFLUENCE_BASE_URL must be an http or https URL")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return Config{}, true, fmt.Errorf("CONFLUENCE_BASE_URL must not include query or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	ca := strings.TrimSpace(getenv("CONFLUENCE_CA_FILE"))
	if shared.TLSVerify && ca != "" {
		if _, err := os.Stat(ca); err != nil {
			return Config{}, true, fmt.Errorf("CONFLUENCE_CA_FILE is not readable")
		}
	}
	return Config{BaseURL: u, CAFile: ca}, true, nil
}
