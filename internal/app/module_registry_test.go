package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeModule struct {
	name string
	err  error
}

func (m fakeModule) Name() string                             { return m.name }
func (m fakeModule) ValidateStaticConfig(config.Shared) error { return m.err }
func (m fakeModule) RegisterTools(*mcp.Server)                {}

func TestRegistryDoesNotLetInvalidModuleBlockValidModule(t *testing.T) {
	var stderr strings.Builder
	registry := NewModuleRegistry(&stderr)
	registry.Add(fakeModule{name: "jira", err: errors.New("bad jira")})
	registry.Add(fakeModule{name: "bitbucket"})

	statuses := registry.Configure(config.Shared{})
	if !statuses["bitbucket"].Enabled {
		t.Fatalf("bitbucket should be enabled: %+v", statuses)
	}
	if statuses["jira"].Enabled || statuses["jira"].DisabledReason == "" {
		t.Fatalf("jira should be disabled with reason: %+v", statuses)
	}
	if strings.Contains(stderr.String(), "TOKEN") || !strings.Contains(stderr.String(), "jira disabled") {
		t.Fatalf("unexpected warning: %q", stderr.String())
	}
}
