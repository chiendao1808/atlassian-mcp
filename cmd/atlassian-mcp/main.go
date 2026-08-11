package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/chiendao1808/atlassian-mcp/internal/app"
	"github.com/chiendao1808/atlassian-mcp/internal/bitbucket"
	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/chiendao1808/atlassian-mcp/internal/confluence"
	"github.com/chiendao1808/atlassian-mcp/internal/jira"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is overridden by release builds with -ldflags "-X main.version=...".
var version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("atlassian-mcp " + version)
		return
	}
	shared, _, err := config.LoadShared(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	jiraModule := jira.NewModule(os.Getenv)
	confluenceModule := confluence.NewModule(os.Getenv)
	server, statuses := app.NewServer(version, shared, os.Stderr, jiraModule, confluenceModule, bitbucket.NewModule(os.Getenv, os.Stderr))
	if !statuses["jira"].Enabled && !statuses["confluence"].Enabled && !statuses["bitbucket"].Enabled {
		fmt.Fprintln(os.Stderr, "no Atlassian business module enabled")
	}
	ctx := context.Background()
	if statuses["jira"].Enabled {
		go jiraModule.AutoAuthenticate(ctx, os.Stderr)
	}
	if statuses["confluence"].Enabled {
		go confluenceModule.AutoAuthenticate(ctx, os.Stderr)
	}
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
