package app

import (
	"fmt"
	"io"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/chiendao1808/atlassian-mcp/internal/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ModuleRegistry struct {
	stderr  io.Writer
	modules []Module
	status  map[string]ModuleStatus
}

func NewModuleRegistry(stderr io.Writer) *ModuleRegistry {
	return &ModuleRegistry{stderr: stderr, status: map[string]ModuleStatus{}}
}

func (r *ModuleRegistry) Add(module Module) {
	r.modules = append(r.modules, module)
}

func (r *ModuleRegistry) Configure(shared config.Shared) map[string]ModuleStatus {
	r.status = map[string]ModuleStatus{}
	for _, module := range r.modules {
		err := module.ValidateStaticConfig(shared)
		if err != nil {
			reason := err.Error()
			r.status[module.Name()] = ModuleStatus{DisabledReason: reason}
			if err != ErrModuleNotRequested && r.stderr != nil {
				fmt.Fprintf(r.stderr, "%s disabled: %s\n", module.Name(), observability.FormatSanitized(reason))
			}
			continue
		}
		r.status[module.Name()] = ModuleStatus{Enabled: true}
	}
	return r.status
}

func (r *ModuleRegistry) RegisterEnabled(server *mcp.Server) {
	for _, module := range r.modules {
		if r.status[module.Name()].Enabled {
			module.RegisterTools(server)
		}
	}
}
