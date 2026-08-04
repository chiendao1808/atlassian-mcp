package jira

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/app"
	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/chiendao1808/atlassian-mcp/internal/jira/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/jira/client"
	"github.com/chiendao1808/atlassian-mcp/internal/jira/tools"
	"github.com/chiendao1808/atlassian-mcp/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Module struct {
	getenv func(string) string
	cfg    Config
	svc    *tools.Service
}

func NewModule(getenv func(string) string) *Module {
	return &Module{getenv: getenv}
}

func (m *Module) Name() string { return "jira" }

func (m *Module) ValidateStaticConfig(shared config.Shared) error {
	cfg, requested, err := LoadConfig(m.getenv, shared)
	if err != nil {
		return err
	}
	if !requested {
		return app.ErrModuleNotRequested
	}
	hc, err := transport.NewHTTPClient(shared, cfg.CAFile)
	if err != nil {
		return err
	}
	m.cfg = cfg
	m.svc = tools.NewService(client.New(cfg.BaseURL.String(), hc, shared.MaxResponseBytes), auth.NewSessionStore(), m.getenv)
	return nil
}

func (m *Module) RegisterTools(server *mcp.Server) {
	if m.svc != nil {
		m.svc.Register(server)
	}
}

// AutoAuthenticate calls jira_authenticate's own logic once at startup when JIRA_USERNAME
// and JIRA_PASSWORD are both already set, so operators who set them do not need an explicit
// jira_authenticate call. It only attempts this when both are present -- an operator who set
// neither sees no extra network call and no log line, matching Section 3.2's startup rule
// (registration itself never depends on this). Failure is a warning on stderr, not fatal:
// tools stay registered and simply keep returning JIRA_NOT_AUTHENTICATED, exactly as if
// jira_authenticate had never been called (ADR-0005).
func (m *Module) AutoAuthenticate(ctx context.Context, stderr io.Writer) {
	if m.svc == nil {
		return
	}
	if strings.TrimSpace(m.getenv("JIRA_USERNAME")) == "" || m.getenv("JIRA_PASSWORD") == "" {
		return
	}
	result := m.svc.Authenticate(ctx, tools.AuthenticateInput{})
	if !result.Success {
		message := "unknown error"
		if result.Error != nil {
			message = result.Error.Message
		}
		fmt.Fprintf(stderr, "jira: automatic authentication using JIRA_USERNAME/JIRA_PASSWORD failed: %s\n", message)
	}
}
