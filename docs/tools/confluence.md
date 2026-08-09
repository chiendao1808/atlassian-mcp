# Confluence Tools

The Confluence module registers exactly these tools after valid static
`CONFLUENCE_BASE_URL` configuration:

| Tool | Access | Notes |
|---|---|---|
| `confluence_authenticate` | Sensitive session setup | Accepts username and password for this process only, or falls back field-by-field to `CONFLUENCE_USERNAME`/`CONFLUENCE_PASSWORD`. It validates with `GET /rest/api/user/current` and activates the session only when Confluence returns a known authenticated user. |
| `confluence_search_content` | Read-only | Runs raw CQL through `GET /rest/api/content/search`. `cql` is required. Optional `cqlcontext`, `expand`, `start`, and `limit` are passed through; omitted `limit` sends `25`. Requires authentication first. |
| `confluence_get_content` | Read-only | Reads one content item with `GET /rest/api/content/{id}`. `contentId` must be one URL path segment. Optional `status`, positive `version`, and caller-selected `expand` are passed through. Requires authentication first. |

The module does not choose a default body representation. Request
`body.storage`, `body.view`, or other Confluence expansions through `expand`.
Upstream JSON is preserved inside the shared result envelope after redaction,
including fields such as `_links` and `_expandable`.

`CONFLUENCE_USERNAME` and `CONFLUENCE_PASSWORD` are optional. If both are set
when the process starts, the server invokes the same authentication logic once
in a background goroutine. Missing either variable sends no request and logs no
warning. Failed automatic authentication logs one sanitized warning to
`stderr`; the tools remain registered and return `CONFLUENCE_NOT_AUTHENTICATED`
until `confluence_authenticate` succeeds.

No Confluence write, attachment, child, comment, space, label, conversion,
crawl, cache, or generic request tool is included in V1.
