package confluence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
)

func TestLoadConfigTreatsAbsentBaseURLAsNotRequested(t *testing.T) {
	cfg, requested, err := LoadConfig(func(string) string { return "" }, config.Shared{})
	if err != nil || requested || cfg.BaseURL != nil {
		t.Fatalf("cfg=%+v requested=%v err=%v", cfg, requested, err)
	}
}

func TestLoadConfigNormalizesAndRejectsBadURLs(t *testing.T) {
	cfg, requested, err := LoadConfig(func(key string) string {
		if key == "CONFLUENCE_BASE_URL" {
			return "https://wiki.internal.example.com/confluence/"
		}
		return ""
	}, config.Shared{})
	if err != nil || !requested {
		t.Fatalf("requested=%v err=%v", requested, err)
	}
	if cfg.BaseURL.String() != "https://wiki.internal.example.com/confluence" {
		t.Fatalf("base URL = %s", cfg.BaseURL)
	}

	_, _, err = LoadConfig(func(key string) string {
		if key == "CONFLUENCE_BASE_URL" {
			return "https://wiki.internal.example.com/confluence?x=1"
		}
		return ""
	}, config.Shared{})
	if err == nil {
		t.Fatal("query component should be rejected")
	}
}

func TestLoadConfigValidatesCAOnlyWhenVerifyTrue(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.pem")
	_, _, err := LoadConfig(func(key string) string {
		switch key {
		case "CONFLUENCE_BASE_URL":
			return "https://wiki.internal.example.com/confluence"
		case "CONFLUENCE_CA_FILE":
			return missing
		default:
			return ""
		}
	}, config.Shared{TLSVerify: false})
	if err != nil {
		t.Fatalf("CA should be ignored when verify=false: %v", err)
	}

	_, _, err = LoadConfig(func(key string) string {
		switch key {
		case "CONFLUENCE_BASE_URL":
			return "https://wiki.internal.example.com/confluence"
		case "CONFLUENCE_CA_FILE":
			return missing
		default:
			return ""
		}
	}, config.Shared{TLSVerify: true})
	if err == nil {
		t.Fatal("missing CA should fail when verify=true")
	}

	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("not checked here"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err = LoadConfig(func(key string) string {
		switch key {
		case "CONFLUENCE_BASE_URL":
			return "https://wiki.internal.example.com/confluence"
		case "CONFLUENCE_CA_FILE":
			return ca
		default:
			return ""
		}
	}, config.Shared{TLSVerify: true})
	if err != nil {
		t.Fatalf("existing CA should pass static config: %v", err)
	}
}
