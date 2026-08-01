package config

import (
	"testing"
	"time"
)

func TestLoadSharedDefaultsAndStrictTLS(t *testing.T) {
	cfg, warnings, err := LoadShared(func(string) string { return "" })
	if err != nil {
		t.Fatalf("LoadShared default error = %v", err)
	}
	if cfg.TLSVerify {
		t.Fatal("default ATLASSIAN_TLS_VERIFY must be false")
	}
	if cfg.ConnectTimeout != 5*time.Second || cfg.RequestTimeout != 60*time.Second {
		t.Fatalf("unexpected timeouts: %+v", cfg)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}

	_, _, err = LoadShared(func(key string) string {
		if key == "ATLASSIAN_TLS_VERIFY" {
			return "maybe"
		}
		return ""
	})
	if err == nil {
		t.Fatal("invalid ATLASSIAN_TLS_VERIFY should fail")
	}
}
