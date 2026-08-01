# Jira Tools

The Jira module registers these tools after valid static `JIRA_BASE_URL` configuration:

| Tool | Access | Notes |
|---|---|---|
| `jira_authenticate` | Sensitive session setup | Accepts username and password for this process only. Review MCP client logging/history policy before use. |
| `jira_get_issue` | Read-only | Reads one issue by `issueIdOrKey`; optional `fields` and `expand` are passed through. |
| `jira_add_issue_comment` | Additive write | Adds a comment; optional visibility uses Jira native `role` or `group`. |
| `jira_update_issue_fields` | Write | Sends native Jira `fields` and/or `update` JSON; optional read-back can return partial success on refresh failure. |
| `jira_transition_issue` | Write | Transitions by ID or exact name; duplicate names are rejected as ambiguous. |

All tool results use the shared envelope: `success`, `service`, `tool`, `data`, `error`, and optional `meta`.