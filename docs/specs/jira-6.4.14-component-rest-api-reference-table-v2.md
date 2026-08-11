# Jira Server 6.4.14 — Component REST API Reference

> **Target:** Jira Server 6.4.14  
> **REST API:** `/rest/api/2`  
> **Reference basis:** Jira REST API 6.4.13 + Jira Java API 6.4.14  
> **Format:** Consolidated table-first reference for planning and implementation

---

## 1. Common conventions

| Item | Value |
|---|---|
| Base URL | `https://<host>/<jira-context>` |
| Example | `https://jira.internal.example.com/jira` |
| REST prefix | `/rest/api/2` |
| Authentication | HTTP Basic Auth over HTTPS |
| MCP authentication | Reuse existing `jira_authenticate` session |
| JSON request header | `Content-Type: application/json` |
| Common response header | `Accept: application/json` |
| REST alias | Do **not** use `/rest/api/latest`; pin `/rest/api/2` |
| Target compatibility | Smoke-test against actual Jira Server 6.4.14 |
| Error format | Preserve Jira `errorMessages` and `errors` |
| Mutation retry | Do not blindly retry POST/PUT/DELETE after ambiguous network failure |

Standalone curl variables:

```bash
export JIRA_BASE='https://jira.internal.example.com/jira'
export JIRA_USER='svc-jira-api'
export JIRA_PASSWORD='***'
```

---

## 2. Component model

| Field | Type | Request | Response | Notes |
|---|---|---:|---:|---|
| `id` | string | No | Yes | Component ID |
| `self` | URI | No | Yes | Component REST URL |
| `name` | string | Yes | Yes | Component name |
| `description` | string | Yes | Yes | Optional description |
| `leadUserName` | string | Yes | No/Derived | Username used when creating/updating lead |
| `lead` | object | No | Yes | Resolved Component lead |
| `assigneeType` | enum | Yes | Yes | Default assignee strategy |
| `assignee` | object | No | Yes | Effective assignee |
| `realAssigneeType` | enum | No | Yes | Effective assignee strategy |
| `realAssignee` | object | No | Yes | Effective resolved assignee |
| `isAssigneeTypeValid` | boolean | Do not expose | Yes | Jira-derived state |
| `project` | string | Create | Yes | Project key |
| `projectId` | integer | Create | Yes | Project numeric ID |

### Assignee type values

| Value | Meaning |
|---|---|
| `PROJECT_DEFAULT` | Use project's default assignee strategy |
| `COMPONENT_LEAD` | Use Component lead |
| `PROJECT_LEAD` | Use Project lead |
| `UNASSIGNED` | Leave issue unassigned if Jira configuration allows it |

---

# 3. Master API matrix

| # | API | Method | Path | Main params/body | Success | Mutation | Suggested MCP tool |
|---:|---|---|---|---|---:|---:|---|
| 1 | Create Component | `POST` | `/rest/api/2/component` | `name`, `description`, `leadUserName`, `assigneeType`, `project` / `projectId` | `201` | Yes | `jira_create_component` |
| 2 | Get Component | `GET` | `/rest/api/2/component/{id}` | Path: `id` | `200` | No | `jira_get_component` |
| 3 | Update Component | `PUT` | `/rest/api/2/component/{id}` | Partial body: `name`, `description`, `leadUserName`, `assigneeType` | `200` | Yes | `jira_update_component` |
| 4 | Delete Component | `DELETE` | `/rest/api/2/component/{id}` | Optional query: `moveIssuesTo` | `204` | Yes, destructive | `jira_delete_component` |
| 5 | Get related issue count | `GET` | `/rest/api/2/component/{id}/relatedIssueCounts` | Path: `id` | `200` | No | `jira_get_component_issue_count` |
| 6 | List project Components | `GET` | `/rest/api/2/project/{projectIdOrKey}/components` | Path: `projectIdOrKey` | `200` | No | `jira_list_project_components` |

---

# 4. Detailed API tables

## 4.1 Create Component

### Request

| Item | Detail |
|---|---|
| Method | `POST` |
| Path | `/rest/api/2/component` |
| Headers | `Accept: application/json`<br>`Content-Type: application/json` |
| Authentication | Basic Auth / authenticated MCP Jira session |
| Retry | Do not blindly retry |
| Permission | Project administration / Component creation permission as enforced by Jira |

### Request body

| Field | Type | Required | Example | Notes |
|---|---|---:|---|---|
| `name` | string | Yes for MCP | `"Backend"` | Component name |
| `description` | string | No | `"Backend services"` | Optional |
| `leadUserName` | string | No | `"fred"` | Existing Jira username |
| `assigneeType` | enum | No | `"COMPONENT_LEAD"` | See assignee type table |
| `project` | string | To verify on 6.4.14 | `"PROJ"` | Project key |
| `projectId` | integer | To verify on 6.4.14 | `10000` | Numeric project ID |

> The generated Jira 6.4.13 REST documentation does not clearly define requiredness or precedence of `project` vs `projectId`. Keep this as a staging contract test before freezing the MCP schema.

### Sample request

```json
{
  "name": "Backend",
  "description": "Backend services",
  "leadUserName": "fred",
  "assigneeType": "COMPONENT_LEAD",
  "project": "PROJ",
  "projectId": 10000
}
```

### Sample curl

```bash
curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  -H 'Content-Type: application/json' \
  -X POST \
  --data '{
    "name": "Backend",
    "description": "Backend services",
    "leadUserName": "fred",
    "assigneeType": "COMPONENT_LEAD",
    "project": "PROJ",
    "projectId": 10000
  }' \
  "$JIRA_BASE/rest/api/2/component"
```

### Response / Error

| HTTP | Meaning | Representative body / handling |
|---:|---|---|
| `201` | Created | Component object |
| `401` | Authentication failure | Normalize to upstream authentication error |
| `403` | Permission denied | Normalize to permission error |
| `404` | Project not found / not visible | Preserve Jira detail |
| Validation failure | Exact status must be verified | Preserve `errorMessages` / `errors` |

Representative success response:

```json
{
  "self": "https://jira.internal.example.com/jira/rest/api/2/component/10100",
  "id": "10100",
  "name": "Backend",
  "description": "Backend services",
  "lead": {
    "name": "fred",
    "displayName": "Fred User",
    "active": true
  },
  "assigneeType": "COMPONENT_LEAD",
  "realAssigneeType": "COMPONENT_LEAD",
  "isAssigneeTypeValid": true,
  "project": "PROJ",
  "projectId": 10000
}
```

---

## 4.2 Get Component

### Request

| Item | Detail |
|---|---|
| Method | `GET` |
| Path | `/rest/api/2/component/{id}` |
| Path parameter | `id` — Component ID |
| Query | None |
| Body | None |
| Success | `200` |
| Retry | Bounded GET retry allowed |

### Sample curl

```bash
curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  "$JIRA_BASE/rest/api/2/component/10100"
```

### Response / Error

| HTTP | Meaning |
|---:|---|
| `200` | Component found |
| `404` | Component missing or not visible |

Representative response:

```json
{
  "self": "https://jira.internal.example.com/jira/rest/api/2/component/10100",
  "id": "10100",
  "name": "Backend",
  "description": "Backend services",
  "assigneeType": "COMPONENT_LEAD",
  "realAssigneeType": "COMPONENT_LEAD",
  "isAssigneeTypeValid": true,
  "project": "PROJ",
  "projectId": 10000
}
```

---

## 4.3 Update Component

### Request

| Item | Detail |
|---|---|
| Method | `PUT` |
| Path | `/rest/api/2/component/{id}` |
| Update semantics | Partial update |
| Omitted fields | Remain unchanged |
| Remove lead | `"leadUserName": ""` |
| Success | `200` |
| Retry | Do not blindly retry |

### Writable body fields

| Field | Type | Required | Behavior |
|---|---|---:|---|
| `name` | string | No | Rename Component |
| `description` | string | No | Replace description |
| `leadUserName` | string | No | Change Component lead |
| `leadUserName=""` | empty string | No | Remove Component lead |
| `assigneeType` | enum | No | Change assignee strategy |

At least one actual update field should be required by the MCP schema.

### Sample request

```json
{
  "name": "Backend Platform",
  "description": "Backend and platform services",
  "assigneeType": "PROJECT_DEFAULT"
}
```

### Sample curl

```bash
curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  -H 'Content-Type: application/json' \
  -X PUT \
  --data '{
    "name": "Backend Platform",
    "assigneeType": "PROJECT_DEFAULT"
  }' \
  "$JIRA_BASE/rest/api/2/component/10100"
```

### Response / Error

| HTTP / Case | Meaning / Handling |
|---|---|
| `200` | Update succeeded |
| `403` | Permission denied |
| `404` | Component missing / hidden |
| Validation error | Preserve Jira body |
| Empty `200` body | Must not fail JSON parsing; staging-test exact 6.4.14 behavior |
| Non-empty `200` body | Preserve Component object if returned |

---

## 4.4 Delete Component

### Request

| Item | Detail |
|---|---|
| Method | `DELETE` |
| Path | `/rest/api/2/component/{id}` |
| Optional query | `moveIssuesTo={targetComponentId}` |
| Success | `204 No Content` |
| Retry | Never blindly retry |
| Mutation | Destructive |

### Parameters

| Name | Location | Type | Required | Meaning |
|---|---|---|---:|---|
| `id` | Path | string | Yes | Component to delete |
| `moveIssuesTo` | Query | string | No | Replacement Component ID |

### Behavior matrix

| Request | Effect |
|---|---|
| `DELETE /component/10100` | Delete Component and remove it from affected Issues |
| `DELETE /component/10100?moveIssuesTo=10200` | Delete Component and replace its references with Component `10200` |

### Sample curl

Without replacement:

```bash
curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  -X DELETE \
  "$JIRA_BASE/rest/api/2/component/10100"
```

With replacement:

```bash
curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  -X DELETE \
  "$JIRA_BASE/rest/api/2/component/10100?moveIssuesTo=10200"
```

### Response / Error

| HTTP | Meaning |
|---:|---|
| `204` | Deleted successfully; no body |
| `403` | Caller cannot delete Component |
| `404` | Component missing / hidden |

### Mandatory staging checks

| Case | Why verify |
|---|---|
| `moveIssuesTo` same as deleted Component | Not sufficiently described |
| Replacement Component missing | Exact validation/status |
| Replacement from another project | Exact Jira 6.4.14 behavior |
| Issues reference Component and no replacement | Confirm Issue mutation behavior |
| Network reset after request | Confirm client does not replay DELETE |

---

## 4.5 Get Component related issue count

### Request

| Item | Detail |
|---|---|
| Method | `GET` |
| Path | `/rest/api/2/component/{id}/relatedIssueCounts` |
| Path parameter | Component ID |
| Success | `200` |
| Mutation | No |

### Sample curl

```bash
curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  "$JIRA_BASE/rest/api/2/component/10100/relatedIssueCounts"
```

### Response / Error

| HTTP | Body / Meaning |
|---:|---|
| `200` | `{"self":"...","issueCount":23}` |
| `404` | Component missing / hidden |

Representative response:

```json
{
  "self": "https://jira.internal.example.com/jira/rest/api/2/component/10100",
  "issueCount": 23
}
```

---

## 4.6 List Project Components

### Request

| Item | Detail |
|---|---|
| Method | `GET` |
| Path | `/rest/api/2/project/{projectIdOrKey}/components` |
| Path parameter | Project key or numeric ID |
| Pagination | None documented |
| Success | `200` |
| Mutation | No |

### Sample curl

By key:

```bash
curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  "$JIRA_BASE/rest/api/2/project/PROJ/components"
```

By ID:

```bash
curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  "$JIRA_BASE/rest/api/2/project/10000/components"
```

### Response / Error

| HTTP / Case | Meaning |
|---|---|
| `200` | Array of Component objects |
| `404` | Project missing / hidden |
| Empty project | Expected `[]`; verify on 6.4.14 |
| Many Components | Endpoint unpaged; enforce MCP response-size limit |

Representative response:

```json
[
  {
    "id": "10100",
    "name": "Backend",
    "project": "PROJ",
    "projectId": 10000
  },
  {
    "id": "10101",
    "name": "Frontend",
    "project": "PROJ",
    "projectId": 10000
  }
]
```

---

# 5. Component usage inside Issue

| Use case | API | Example |
|---|---|---|
| Read Issue Components | `GET /rest/api/2/issue/{issueIdOrKey}?fields=components` | Existing `jira_get_issue` |
| Set Issue Components | `PUT /rest/api/2/issue/{issueIdOrKey}` | Existing `jira_update_issue_fields` |
| Check field editability | `GET /rest/api/2/issue/{issueIdOrKey}/editmeta` | Validate whether `components` can be edited |

### Issue update example

```json
{
  "fields": {
    "components": [
      { "id": "10100" },
      { "id": "10101" }
    ]
  }
}
```

No separate "attach Component" MCP tool is required unless intentionally added as a convenience wrapper.

---

# 6. MCP tool planning matrix

| Tool | Input | REST mapping | Type | Approval | Key tests |
|---|---|---|---|---|---|
| `jira_list_project_components` | `projectIdOrKey` | `GET /project/{projectIdOrKey}/components` | Read | No | key/ID, empty, hidden, response cap |
| `jira_get_component` | `componentId` | `GET /component/{id}` | Read | No | existing, missing, hidden |
| `jira_create_component` | project selector, `name`, optional metadata | `POST /component` | Write | Yes | project/projectId semantics, duplicate, invalid lead |
| `jira_update_component` | `componentId` + ≥1 field | `PUT /component/{id}` | Write | Yes | partial update, clear lead, empty body |
| `jira_delete_component` | `componentId`, optional `moveIssuesTo` | `DELETE /component/{id}` | Destructive | Yes | replacement behavior, one DELETE |
| `jira_get_component_issue_count` | `componentId` | `GET /component/{id}/relatedIssueCounts` | Read | No | zero/multiple/missing |

---

# 7. Error mapping

| HTTP / Condition | MCP category | Notes |
|---|---|---|
| `400` | `VALIDATION_ERROR` | When target Jira returns it |
| `401` | `UPSTREAM_AUTHENTICATION_FAILED` | Jira session invalid |
| `403` | `UPSTREAM_PERMISSION_DENIED` | Permission denied |
| `404` | `UPSTREAM_NOT_FOUND` | Missing or intentionally hidden |
| `5xx` | `UPSTREAM_SERVER_ERROR` | Preserve bounded detail |
| Timeout | `UPSTREAM_TIMEOUT` | GET retry only |
| Connection failure | `UPSTREAM_UNREACHABLE` | Mutation replay prohibited |
| Oversized response | `RESPONSE_TOO_LARGE` | Relevant to project Component list |

Standard Jira error shape:

```json
{
  "errorMessages": [
    "A validation error occurred."
  ],
  "errors": {
    "name": "Component name is invalid."
  }
}
```

---

# 8. Contract-test checklist

| API | Required Jira 6.4.14 tests |
|---|---|
| Create | project only; projectId only; both matching; both conflicting; neither; duplicate name; empty name; invalid lead; each assigneeType |
| Get | existing; no lead; invalid ID; hidden Component; assignee fallback |
| Update | rename; description; change/remove lead; assigneeType; empty body; duplicate name; response-body behavior |
| Delete | unused; used/no replacement; valid replacement; same replacement ID; missing replacement; cross-project replacement; one DELETE only |
| Issue count | zero; multiple; missing; permission visibility |
| List | project key; numeric ID; empty array; many Components; hidden project; no pagination |

---

# 9. Official source references

| Source | URL | Purpose |
|---|---|---|
| Jira REST API 6.4.13 | `https://docs.atlassian.com/software/jira/docs/api/REST/6.4.13/` | Primary REST Component API |
| Jira Java API 6.4.14 — `ProjectComponentManager` | `https://docs.atlassian.com/software/jira/docs/api/6.4.14/com/atlassian/jira/bc/project/component/ProjectComponentManager.html` | Cross-check Component domain behavior |

---

# 10. Open implementation decisions

| Item | Status | Required action |
|---|---|---|
| `project` vs `projectId` on create | Not fully specified by published REST page | Contract-test Jira 6.4.14 before freezing schema |
| PUT `200` response body | Published page not explicit enough | Record actual 6.4.14 response |
| Cross-project `moveIssuesTo` | Not sufficiently documented | Contract-test |
| Duplicate Component name behavior | Validation details not explicit | Contract-test |
| `UNASSIGNED` validity | Depends on Jira configuration | Preserve Jira result/error |
