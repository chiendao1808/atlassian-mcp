package bitbucket

import (
	"io"

	"github.com/chiendao1808/atlassian-mcp/internal/app"
	bbclient "github.com/chiendao1808/atlassian-mcp/internal/bitbucket/client"
	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/chiendao1808/atlassian-mcp/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Module struct {
	getenv func(string) string
	stderr io.Writer
	cfg    Config
	client *bbclient.Client
}

func NewModule(getenv func(string) string, stderr ...io.Writer) *Module {
	var out io.Writer
	if len(stderr) > 0 {
		out = stderr[0]
	}
	return &Module{getenv: getenv, stderr: out}
}

func (m *Module) Name() string { return "bitbucket" }

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
	m.client = bbclient.New(cfg.BaseURL.String(), cfg.ProjectKey, cfg.Token, hc, shared.MaxResponseBytes, bbclient.Options{Stderr: m.stderr})
	return nil
}

func (m *Module) RegisterTools(*mcp.Server) {
	// Bitbucket business tools are implemented by follow-up tasks.
}
