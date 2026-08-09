package app_test

import (
	"io"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/app"
	"github.com/chiendao1808/atlassian-mcp/internal/bitbucket"
	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/chiendao1808/atlassian-mcp/internal/confluence"
	"github.com/chiendao1808/atlassian-mcp/internal/jira"
)

func TestNewServerKeepsBitbucketEnabledWhenJiraConfigIsInvalid(t *testing.T) {
	_, statuses := app.NewServer("test", config.Shared{}, io.Discard,
		jira.NewModule(env(map[string]string{"JIRA_BASE_URL": "://bad"})),
		bitbucket.NewModule(env(map[string]string{
			"BITBUCKET_BASE_URL":     "https://bitbucket.internal.example.com/bitbucket",
			"BITBUCKET_PROJECT_KEY":  "PRJ",
			"BITBUCKET_BEARER_TOKEN": "token",
		})),
	)
	if !statuses["bitbucket"].Enabled || statuses["jira"].Enabled {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestNewServerKeepsJiraEnabledWhenBitbucketConfigIsInvalid(t *testing.T) {
	_, statuses := app.NewServer("test", config.Shared{}, io.Discard,
		jira.NewModule(env(map[string]string{"JIRA_BASE_URL": "https://jira.internal.example.com/jira"})),
		bitbucket.NewModule(env(map[string]string{"BITBUCKET_BASE_URL": "://bad"})),
	)
	if !statuses["jira"].Enabled || statuses["bitbucket"].Enabled {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestNewServerKeepsJiraEnabledWhenConfluenceConfigIsInvalid(t *testing.T) {
	_, statuses := app.NewServer("test", config.Shared{}, io.Discard,
		jira.NewModule(env(map[string]string{"JIRA_BASE_URL": "https://jira.internal.example.com/jira"})),
		confluence.NewModule(env(map[string]string{"CONFLUENCE_BASE_URL": "://bad"})),
	)
	if !statuses["jira"].Enabled || statuses["confluence"].Enabled {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestNewServerKeepsConfluenceEnabledWhenJiraConfigIsInvalid(t *testing.T) {
	_, statuses := app.NewServer("test", config.Shared{}, io.Discard,
		jira.NewModule(env(map[string]string{"JIRA_BASE_URL": "://bad"})),
		confluence.NewModule(env(map[string]string{"CONFLUENCE_BASE_URL": "https://wiki.internal.example.com/confluence"})),
	)
	if !statuses["confluence"].Enabled || statuses["jira"].Enabled {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func env(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
