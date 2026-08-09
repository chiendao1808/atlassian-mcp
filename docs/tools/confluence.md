# Confluence Tools

The Confluence module registers exactly these tools after valid static
`CONFLUENCE_BASE_URL` configuration:

| Tool | Access | Notes |
|---|---|---|
| `confluence_authenticate` | Sensitive session setup | Accepts username and password for this process only, or falls back field-by-field to `CONFLUENCE_USERNAME`/`CONFLUENCE_PASSWORD`. It validates with `GET /rest/api/user/current` and activates the session only when Confluence returns a known authenticated user. |
| `confluence_search_content` | Read-only | Runs raw CQL through `GET /rest/api/content/search`. `cql` is required. Optional `cqlcontext`, `expand`, `start`, and `limit` are passed through; omitted `limit` sends `25`. Requires authentication first. |
| `confluence_get_content` | Read-only | Reads one content item with `GET /rest/api/content/{id}`. `contentId` must be one URL path segment. Optional `status`, positive `version`, and caller-selected `expand` are passed through. Requires authentication first. |
| `confluence_list_content` | Read-only | Lists content with `GET /rest/api/content`. Optional `type` (`page` or `blogpost`), `spaceKey`, `title`, `status`, `postingDay` (`yyyy-mm-dd`), `expand`, `start`, and `limit` are passed through; omitted `limit` sends `25`, while omitted `type` leaves Confluence's native default. Requires authentication first. |
| `confluence_list_content_properties` | Read-only | Lists content properties with `GET /rest/api/content/{id}/property`. `contentId` must be one URL path segment. Optional `expand`, `start`, and `limit` are passed through; omitted `limit` sends the documented property default `10`. Requires authentication first. |
| `confluence_get_content_property` | Read-only | Reads one content property with `GET /rest/api/content/{id}/property/{key}`. `contentId` and `key` must each be one URL path segment. Optional `expand` is passed through. Requires authentication first. |
| `confluence_list_spaces` | Read-only | Lists visible spaces with `GET /rest/api/space`. Optional `spaceKey`, `type` (`global` or `personal`), `status` (`current` or `archived`), `label`, `expand`, `start`, and `limit` are passed through; omitted `limit` sends `25`. Requires authentication first. |
| `confluence_get_space` | Read-only | Reads one space with `GET /rest/api/space/{spaceKey}`. `spaceKey` must be one URL path segment. Optional `expand` is passed through. Requires authentication first. |
| `confluence_list_space_content` | Read-only | Lists grouped content for one space with `GET /rest/api/space/{spaceKey}/content`. `spaceKey` must be one URL path segment. Optional `depth` (`all` or `root`), `expand`, `start`, and `limit` are passed through; omitted `limit` sends `25`, while omitted `depth` leaves Confluence's native default. Requires authentication first. |

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

No Confluence write, typed space-content, history, macro, child, comment,
descendant, attachment, label, restriction, space-property, conversion, crawl,
cache, or generic request tool is included in V1. Content Property support is
limited to the two GET endpoints; no create, update, or delete property tool is
registered.
