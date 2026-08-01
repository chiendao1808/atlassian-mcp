package bitbucket

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
)

func TestLoadConfigTreatsAbsentBitbucketConfigAsNotRequested(t *testing.T) {
	cfg, requested, err := LoadConfig(func(string) string { return "" }, config.Shared{})
	if err != nil || requested || cfg.BaseURL != nil {
		t.Fatalf("cfg=%+v requested=%v err=%v", cfg, requested, err)
	}
}

func TestLoadConfigRequiresCompleteBitbucketConfig(t *testing.T) {
	_, requested, err := LoadConfig(func(key string) string {
		if key == "BITBUCKET_BASE_URL" {
			return "https://bitbucket.internal.example.com/bitbucket"
		}
		return ""
	}, config.Shared{})
	if !requested || err == nil {
		t.Fatalf("partial config should be requested and invalid: requested=%v err=%v", requested, err)
	}
}

func TestLoadConfigNormalizesBaseURLAndOptionalValues(t *testing.T) {
	cfg, requested, err := LoadConfig(func(key string) string {
		switch key {
		case "BITBUCKET_BASE_URL":
			return "https://bitbucket.internal.example.com/bitbucket/"
		case "BITBUCKET_PROJECT_KEY":
			return " PRJ "
		case "BITBUCKET_BEARER_TOKEN":
			return "token-sentinel"
		case "BITBUCKET_USER_SLUG":
			return " user-slug "
		default:
			return ""
		}
	}, config.Shared{})
	if err != nil || !requested {
		t.Fatalf("requested=%v err=%v", requested, err)
	}
	if cfg.BaseURL.String() != "https://bitbucket.internal.example.com/bitbucket" {
		t.Fatalf("base URL = %s", cfg.BaseURL)
	}
	if cfg.ProjectKey != "PRJ" || cfg.Token != "token-sentinel" || cfg.UserSlug != "user-slug" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadConfigRejectsQueryAndFragment(t *testing.T) {
	for _, raw := range []string{
		"https://bitbucket.internal.example.com/bitbucket?x=1",
		"https://bitbucket.internal.example.com/bitbucket#frag",
	} {
		_, _, err := LoadConfig(func(key string) string {
			switch key {
			case "BITBUCKET_BASE_URL":
				return raw
			case "BITBUCKET_PROJECT_KEY":
				return "PRJ"
			case "BITBUCKET_BEARER_TOKEN":
				return "token"
			default:
				return ""
			}
		}, config.Shared{})
		if err == nil {
			t.Fatalf("%s should be rejected", raw)
		}
	}
}

func TestLoadConfigValidatesCAOnlyWhenVerifyTrue(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.pem")
	getenv := func(key string) string {
		switch key {
		case "BITBUCKET_BASE_URL":
			return "https://bitbucket.internal.example.com/bitbucket"
		case "BITBUCKET_PROJECT_KEY":
			return "PRJ"
		case "BITBUCKET_BEARER_TOKEN":
			return "token"
		case "BITBUCKET_CA_FILE":
			return missing
		default:
			return ""
		}
	}
	if _, _, err := LoadConfig(getenv, config.Shared{TLSVerify: false}); err != nil {
		t.Fatalf("CA should be ignored when verify=false: %v", err)
	}
	if _, _, err := LoadConfig(getenv, config.Shared{TLSVerify: true}); err == nil {
		t.Fatal("missing CA should fail when verify=true")
	}

	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("not checked here"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadConfig(func(key string) string {
		if key == "BITBUCKET_CA_FILE" {
			return ca
		}
		return getenv(key)
	}, config.Shared{TLSVerify: true})
	if err != nil {
		t.Fatalf("existing CA should pass static config: %v", err)
	}
}
