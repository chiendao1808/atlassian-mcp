package confluence

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/app"
	"github.com/chiendao1808/atlassian-mcp/internal/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/chiendao1808/atlassian-mcp/internal/confluence/client"
	"github.com/chiendao1808/atlassian-mcp/internal/confluence/tools"
	"github.com/chiendao1808/atlassian-mcp/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Module wires Confluence static configuration, REST access, and MCP tools.
type Module struct {
	getenv func(string) string
	cfg    Config
	svc    *tools.Service
}

// NewModule returns a Confluence module that reads process environment through getenv.
func NewModule(getenv func(string) string) *Module {
	return &Module{getenv: getenv}
}

// Name returns the registry key for Confluence status and warnings.
func (m *Module) Name() string { return "confluence" }

// ValidateStaticConfig enables Confluence only when its static settings are valid.
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

// RegisterTools registers the exact Confluence V1 toolset when the module is enabled.
func (m *Module) RegisterTools(server *mcp.Server) {
	if m.svc != nil {
		m.svc.Register(server)
	}
}

// AutoAuthenticate initializes an empty Confluence session from env credentials without blocking startup.
func (m *Module) AutoAuthenticate(ctx context.Context, stderr io.Writer) {
	if m.svc == nil {
		return
	}
	if strings.TrimSpace(m.getenv("CONFLUENCE_USERNAME")) == "" || m.getenv("CONFLUENCE_PASSWORD") == "" {
		return
	}
	if _, err := m.svc.Store().Snapshot(); err == nil {
		return
	}
	result := m.svc.AuthenticateIfSessionUnchanged(ctx, tools.AuthenticateInput{}, nil)
	if !result.Success {
		message := "unknown error"
		if result.Error != nil {
			message = result.Error.Message
		}
		fmt.Fprintf(stderr, "confluence: automatic authentication using CONFLUENCE_USERNAME/CONFLUENCE_PASSWORD failed: %s\n", message)
	}
}
