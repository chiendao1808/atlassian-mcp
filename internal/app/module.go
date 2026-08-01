package app

import (
	"errors"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var ErrModuleNotRequested = errors.New("module not requested")

type Module interface {
	Name() string
	ValidateStaticConfig(config.Shared) error
	RegisterTools(*mcp.Server)
}

type ModuleStatus struct {
	Enabled        bool
	DisabledReason string
}
