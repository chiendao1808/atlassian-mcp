package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/chiendao1808/atlassian-mcp/internal/app"
	"github.com/chiendao1808/atlassian-mcp/internal/bitbucket"
	"github.com/chiendao1808/atlassian-mcp/internal/config"
	"github.com/chiendao1808/atlassian-mcp/internal/jira"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.1.0"

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
	server, statuses := app.NewServer(version, shared, os.Stderr, jira.NewModule(os.Getenv), bitbucket.NewModule(os.Getenv, os.Stderr))
	if !statuses["jira"].Enabled && !statuses["bitbucket"].Enabled {
		fmt.Fprintln(os.Stderr, "no Atlassian business module enabled")
	}
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
