package jira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
)

func TestModuleUsesSharedTLSClientConfig(t *testing.T) {
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("not pem"), 0600); err != nil {
		t.Fatal(err)
	}
	module := NewModule(func(key string) string {
		switch key {
		case "JIRA_BASE_URL":
			return "https://jira.internal.example.com/jira"
		case "JIRA_CA_FILE":
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
