package jira

import (
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
	m.svc = tools.NewService(client.New(cfg.BaseURL.String(), hc, shared.MaxResponseBytes), auth.NewSessionStore())
	return nil
}

func (m *Module) RegisterTools(server *mcp.Server) {
	if m.svc != nil {
		m.svc.Register(server)
	}
}
