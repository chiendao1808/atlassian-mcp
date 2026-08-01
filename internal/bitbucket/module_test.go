package bitbucket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
)

func TestModuleBuildsClientWhenConfigured(t *testing.T) {
	module := NewModule(func(key string) string {
		switch key {
		case "BITBUCKET_BASE_URL":
			return "https://bitbucket.internal.example.com/bitbucket"
		case "BITBUCKET_PROJECT_KEY":
			return "PRJ"
		case "BITBUCKET_BEARER_TOKEN":
			return "token"
		default:
			return ""
		}
	})
	if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
		t.Fatal(err)
	}
	if module.client == nil {
		t.Fatal("configured module should build a Bitbucket REST client")
	}
}

func TestModuleUsesSharedTLSClientConfig(t *testing.T) {
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("not pem"), 0600); err != nil {
		t.Fatal(err)
	}
	module := NewModule(func(key string) string {
		switch key {
		case "BITBUCKET_BASE_URL":
			return "https://bitbucket.internal.example.com/bitbucket"
		case "BITBUCKET_PROJECT_KEY":
			return "PRJ"
		case "BITBUCKET_BEARER_TOKEN":
			return "token"
		case "BITBUCKET_CA_FILE":
			return ca
		default:
			return ""
		}
	})
	err := module.ValidateStaticConfig(config.Shared{TLSVerify: true})
	if err == nil || !strings.Contains(err.Error(), "PEM") {
		t.Fatalf("expected shared transport CA validation error, got %v", err)
	}
}
