# Atlassian MCP

This repository contains the initial Go implementation for `atlassian-mcp` plus the planning artifacts that define the remaining release work.


## Canonical repository

Implementation repository:

```text
https://github.com/chiendao1808/atlassian-mcp.git
```

Raw installer URL patterns:

```text
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.sh
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.ps1
```

Use a release tag or full commit SHA for `<ref>` in production. Installer examples use the same repository as the default source:

```text
--source-repo-url https://github.com/chiendao1808/atlassian-mcp.git
-SourceRepoUrl 'https://github.com/chiendao1808/atlassian-mcp.git'
```


## Build and run

```bash
go test ./...
go build ./cmd/atlassian-mcp
./atlassian-mcp --version
```

Runtime uses MCP stdio. Protocol messages are written to `stdout`; logs and startup warnings are written to `stderr`.

Jira tools are registered when `JIRA_BASE_URL` is statically valid. Call `jira_authenticate` once per MCP process session before using Jira issue tools.

Bitbucket configuration is isolated from Jira startup. The Bitbucket module includes the shared REST client foundation for later repository and pull-request tools, but business tools are implemented by the follow-up Bitbucket tasks.

## Contents

```text
atlassian-mcp/
├── README.md
├── CHECKSUMS.sha256
├── manifest.json
├── docs/
│   └── atlassian-mcp-implementation-plan.md
└── references/
    └── jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md
```

## Current naming only

The plan uses only:

- `atlassian-mcp`
- `scripts/install-from-remote.sh`
- `scripts/install-from-remote.ps1`
- `--source-repo-url` / `-SourceRepoUrl`
- `--source-ref` / `-SourceRef`
- explicit `jira-*` and `bitbucket-*` target parameters

No draft aliases or backward-compatibility layer is included because implementation has not started.

## Why installer scripts are not included

Earlier generated installer scripts targeted the previous Bitbucket-only draft and contain superseded names. They are intentionally excluded from this planning bundle. Tasks 15 and 16 define the final installers to implement from scratch.

## Installer location in the implementation repository

The final implementation repository must contain the installers at these exact paths:

```text
scripts/install-from-remote.sh
scripts/install-from-remote.ps1
```

They can be fetched from GitHub through:

```text
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.sh
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.ps1
```

`<ref>` should be a release tag or full commit SHA for production. The downloaded installer remains provider-neutral: `--source-repo-url` or `-SourceRepoUrl` may point to any supported Git remote.

These scripts are not included in this planning-only bundle. Tasks 15 and 16 create them during implementation.
