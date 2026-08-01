package app

import (
	"io"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewServer(version string, shared config.Shared, stderr io.Writer, modules ...Module) (*mcp.Server, map[string]ModuleStatus) {
	server := mcp.NewServer(&mcp.Implementation{Name: "atlassian-mcp", Version: version}, nil)
	registry := NewModuleRegistry(stderr)
	for _, module := range modules {
		registry.Add(module)
	}
	statuses := registry.Configure(shared)
	registry.RegisterEnabled(server)
	return server, statuses
}
