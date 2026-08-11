package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/jira/client"
	"github.com/chiendao1808/atlassian-mcp/internal/observability"
	"github.com/chiendao1808/atlassian-mcp/internal/result"
)

// Service owns the Jira tool handlers and the process-local authenticated session.
type Service struct {
	client *client.Client
	store  *auth.SessionStore
	getenv func(string) string
}

// NewService binds Jira REST access, session storage, and env var lookup (used as a
// jira_authenticate fallback) for MCP tool execution.
func NewService(client *client.Client, store *auth.SessionStore, getenv func(string) string) *Service {
	return &Service{client: client, store: store, getenv: getenv}
}

// Store exposes the session store for module wiring and focused tests.
func (s *Service) Store() *auth.SessionStore { return s.store }

// AuthenticateInput carries the Jira credential payload accepted by the toolset. Either
// field may be omitted to fall back to JIRA_USERNAME/JIRA_PASSWORD.
type AuthenticateInput struct {
	Username string `json:"username,omitempty" jsonschema:"Jira username; falls back to JIRA_USERNAME if omitted"`
	Password string `json:"password,omitempty" jsonschema:"Sensitive Jira password; falls back to JIRA_PASSWORD if omitted"`
}

// GetIssueInput selects one Jira issue and optional native Jira query expansions.
type GetIssueInput struct {
	IssueIDOrKey string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	Fields       []string `json:"fields,omitempty" jsonschema:"Optional Jira fields query values"`
	Expand       []string `json:"expand,omitempty" jsonschema:"Optional Jira expand query values"`
}

// Visibility is Jira's native role/group comment visibility shape.
type Visibility struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// AddCommentInput posts one Jira comment without automatic replay on transport failure.
type AddCommentInput struct {
	IssueIDOrKey string      `json:"issueIdOrKey"`
	Body         string      `json:"body"`
	Visibility   *Visibility `json:"visibility,omitempty"`
}

// UpdateIssueInput passes Jira fields/update JSON through unchanged and can refresh the issue after mutation.
type UpdateIssueInput struct {
	IssueIDOrKey string         `json:"issueIdOrKey"`
	Fields       map[string]any `json:"fields,omitempty"`
	Update       map[string]any `json:"update,omitempty"`
	ReturnIssue  bool           `json:"returnIssue,omitempty"`
	ReturnFields []string       `json:"returnFields,omitempty"`
	ReturnExpand []string       `json:"returnExpand,omitempty"`
}

// TransitionIssueInput selects one Jira transition and optionally carries native transition-screen updates.
type TransitionIssueInput struct {
	IssueIDOrKey   string         `json:"issueIdOrKey"`
	TransitionID   string         `json:"transitionId,omitempty"`
	TransitionName string         `json:"transitionName,omitempty"`
	Fields         map[string]any `json:"fields,omitempty"`
	Update         map[string]any `json:"update,omitempty"`
	ReturnIssue    bool           `json:"returnIssue,omitempty"`
	ReturnFields   []string       `json:"returnFields,omitempty"`
	ReturnExpand   []string       `json:"returnExpand,omitempty"`
}

// CreateIssueInput carries native Jira fields/update JSON for creating exactly one new issue.
type CreateIssueInput struct {
	Fields map[string]any `json:"fields" jsonschema:"Native Jira fields payload for the new issue (required)"`
	Update map[string]any `json:"update,omitempty" jsonschema:"Optional native Jira update operations applied at creation time"`
}

// BulkCreateIssuesInput carries one or more native Jira {fields, update?} payloads for Jira's bulk
// create endpoint, which accepts and reports per-row outcomes in a single upstream call.
type BulkCreateIssuesInput struct {
	IssueUpdates []map[string]any `json:"issueUpdates" jsonschema:"One or more native Jira {fields, update?} payloads, one per issue to create (required, non-empty)"`
}

// DeleteIssueInput selects one Jira issue to permanently delete and whether its subtasks go with it.
type DeleteIssueInput struct {
	IssueIDOrKey   string `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	DeleteSubtasks bool   `json:"deleteSubtasks,omitempty" jsonschema:"Also delete this issue's subtasks; defaults to false"`
}

// AssignIssueInput models Jira's two-state assignee semantics (decision D-A): Name empty/omitted
// means unassign, Name non-empty means assign to that user. It also carries the same optional
// post-mutation read-back triad as jira_update_issue_fields/jira_transition_issue.
type AssignIssueInput struct {
	IssueIDOrKey string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	Name         string   `json:"name,omitempty" jsonschema:"Username to assign; empty or omitted unassigns the issue"`
	ReturnIssue  bool     `json:"returnIssue,omitempty" jsonschema:"Read the issue back after assignment"`
	ReturnFields []string `json:"returnFields,omitempty" jsonschema:"Optional Jira fields query values for the read-back; requires returnIssue=true"`
	ReturnExpand []string `json:"returnExpand,omitempty" jsonschema:"Optional Jira expand query values for the read-back; requires returnIssue=true"`
}

// SearchIssuesInput carries a JQL query plus optional pagination/shape controls for Jira's POST
// search endpoint (chosen over GET to avoid URL-length limits on long JQL, decision D-4).
type SearchIssuesInput struct {
	JQL           string   `json:"jql" jsonschema:"JQL query string (required)"`
	StartAt       *int     `json:"startAt,omitempty" jsonschema:"Pagination offset; omitted means Jira's default"`
	MaxResults    *int     `json:"maxResults,omitempty" jsonschema:"Maximum results to return; omitted means Jira's default"`
	Fields        []string `json:"fields,omitempty" jsonschema:"Optional Jira fields to include per issue"`
	Expand        []string `json:"expand,omitempty" jsonschema:"Optional Jira expand values"`
	ValidateQuery *bool    `json:"validateQuery,omitempty" jsonschema:"Whether Jira should validate the JQL before searching; omitted means Jira's default"`
}

// ListIssueCommentsInput selects one Jira issue's comments with optional pagination, ordering, and
// expand controls. OrderBy is forwarded to Jira only when the caller sets it to a non-blank value: per
// the spec, whether Jira 6.4.14 honors orderBy on this endpoint at all is itself patch-dependent, so an
// unset value is never defaulted or guessed at (see docs/tools/jira.md for the caveat callers should
// expect).
type ListIssueCommentsInput struct {
	IssueIDOrKey string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	StartAt      *int     `json:"startAt,omitempty" jsonschema:"Pagination offset; omitted means Jira's default"`
	MaxResults   *int     `json:"maxResults,omitempty" jsonschema:"Maximum results to return; omitted means Jira's default"`
	OrderBy      string   `json:"orderBy,omitempty" jsonschema:"Optional Jira orderBy value, sent only when non-blank; support is patch-dependent on Jira 6.4.14"`
	Expand       []string `json:"expand,omitempty" jsonschema:"Optional Jira expand query values"`
}

// UpdateIssueCommentInput edits one existing Jira comment's body (and optionally its visibility) and
// carries the same optional post-mutation read-back triad as AssignIssue/UpdateIssueFields. Expand
// mirrors Jira's optional expand query on this endpoint for schema parity, but is not currently
// forwarded on the wire: the shared client's PutJSON has no query-parameter support to carry it (that
// is Group 0 client scope, not this dispatch's), and the approved request shape for this tool sends
// only {body, visibility?} in the body. A future client extension (a PutJSONQuery mirroring the
// existing PostJSONQuery) would be required to honor it.
type UpdateIssueCommentInput struct {
	IssueIDOrKey string      `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	CommentID    string      `json:"commentId" jsonschema:"Jira comment ID (required)"`
	Body         string      `json:"body" jsonschema:"Updated comment body text (required)"`
	Visibility   *Visibility `json:"visibility,omitempty"`
	Expand       []string    `json:"expand,omitempty" jsonschema:"Optional Jira expand query values; accepted for schema parity, not currently forwarded to Jira (see UpdateIssueComment doc comment)"`
	ReturnIssue  bool        `json:"returnIssue,omitempty" jsonschema:"Read the issue back after updating the comment"`
	ReturnFields []string    `json:"returnFields,omitempty" jsonschema:"Optional Jira fields query values for the read-back; requires returnIssue=true"`
	ReturnExpand []string    `json:"returnExpand,omitempty" jsonschema:"Optional Jira expand query values for the read-back; requires returnIssue=true"`
}

// DeleteIssueCommentInput permanently deletes one Jira comment and carries the same optional
// post-mutation read-back triad as AssignIssue: unlike DeleteIssue, the parent issue still exists
// after this call, so a read-back has a natural anchor.
type DeleteIssueCommentInput struct {
	IssueIDOrKey string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	CommentID    string   `json:"commentId" jsonschema:"Jira comment ID (required)"`
	ReturnIssue  bool     `json:"returnIssue,omitempty" jsonschema:"Read the issue back after deleting the comment"`
	ReturnFields []string `json:"returnFields,omitempty" jsonschema:"Optional Jira fields query values for the read-back; requires returnIssue=true"`
	ReturnExpand []string `json:"returnExpand,omitempty" jsonschema:"Optional Jira expand query values for the read-back; requires returnIssue=true"`
}

// ListIssueTransitionsInput lists the transitions currently available on one Jira issue. Expand
// defaults to "transitions.fields" (the spec's own recommendation for discovering which fields a
// transition screen requires) whenever the caller omits it; a caller-supplied Expand always replaces
// that default outright rather than being merged with it.
type ListIssueTransitionsInput struct {
	IssueIDOrKey string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	TransitionID string   `json:"transitionId,omitempty" jsonschema:"Optional Jira transition ID to filter the listing to a single transition"`
	Expand       []string `json:"expand,omitempty" jsonschema:"Optional Jira expand query values; defaults to transitions.fields when omitted"`
}

// AddIssueAttachmentInput uploads a single file attachment (base64-encoded, since MCP tool inputs are
// JSON-only with no raw file handle) to one Jira issue, then optionally reads the issue back via the
// same triad/guard as AssignIssue/UpdateIssueComment. Multi-file upload is an explicit out-of-scope
// follow-up (decision D-B) -- one file per call.
type AddIssueAttachmentInput struct {
	IssueIDOrKey  string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	Filename      string   `json:"filename" jsonschema:"Attachment file name (required)"`
	ContentBase64 string   `json:"contentBase64" jsonschema:"Base64-encoded file content (required); MCP tool inputs carry no raw file handle"`
	ReturnIssue   bool     `json:"returnIssue,omitempty" jsonschema:"Read the issue back after uploading the attachment"`
	ReturnFields  []string `json:"returnFields,omitempty" jsonschema:"Optional Jira fields query values for the read-back; requires returnIssue=true"`
	ReturnExpand  []string `json:"returnExpand,omitempty" jsonschema:"Optional Jira expand query values for the read-back; requires returnIssue=true"`
}

// DeleteIssueAttachmentInput permanently deletes one Jira attachment by ID. Its REST path
// (/attachment/{id}) carries no issue key at all, so unlike the comment/issue delete tools there is no
// ReturnIssue/ReturnFields/ReturnExpand triad here -- there is no anchor issue to refresh (plan
// decision D-R's exclusion list).
type DeleteIssueAttachmentInput struct {
	AttachmentID string `json:"attachmentId" jsonschema:"Jira attachment ID (required); this endpoint has no issue key in its path"`
}

// ListIssueWorklogsInput selects one Jira issue's worklog listing. Read-only: no post-mutation
// read-back triad applies (the plan lists no extra pagination/query params for this endpoint).
type ListIssueWorklogsInput struct {
	IssueIDOrKey string `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
}

// AddIssueWorklogInput records time spent on one Jira issue and optionally adjusts the remaining
// estimate. AdjustEstimate, when set, must be exactly one of Jira's four accepted values: new, leave,
// manual, auto. NewEstimate/ReduceBy are meaningful only alongside a set AdjustEstimate and, per Jira
// semantics, always travel on the query string -- never in the JSON body -- alongside adjustEstimate
// itself; comment/started/timeSpentSeconds are the only fields sent in the body. It also carries the
// same optional post-mutation read-back triad as AssignIssue/UpdateIssueComment.
type AddIssueWorklogInput struct {
	IssueIDOrKey     string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	TimeSpentSeconds int      `json:"timeSpentSeconds" jsonschema:"Time spent in seconds (required, must be > 0)"`
	Comment          string   `json:"comment,omitempty" jsonschema:"Optional worklog comment"`
	Started          string   `json:"started,omitempty" jsonschema:"Optional Jira-format start datetime string, passed through as-is"`
	AdjustEstimate   string   `json:"adjustEstimate,omitempty" jsonschema:"Optional estimate adjustment mode: new, leave, manual, or auto"`
	NewEstimate      string   `json:"newEstimate,omitempty" jsonschema:"New remaining estimate; sent as a query parameter only when adjustEstimate is set"`
	ReduceBy         string   `json:"reduceBy,omitempty" jsonschema:"Amount to reduce the remaining estimate by; sent as a query parameter only when adjustEstimate is set"`
	ReturnIssue      bool     `json:"returnIssue,omitempty" jsonschema:"Read the issue back after adding the worklog"`
	ReturnFields     []string `json:"returnFields,omitempty" jsonschema:"Optional Jira fields query values for the read-back; requires returnIssue=true"`
	ReturnExpand     []string `json:"returnExpand,omitempty" jsonschema:"Optional Jira expand query values for the read-back; requires returnIssue=true"`
}

// GetIssueWatchersInput selects one Jira issue's watcher listing. Read-only: no post-mutation
// read-back triad applies here.
type GetIssueWatchersInput struct {
	IssueIDOrKey string `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
}

// AddIssueWatcherInput adds one Jira user as a watcher on an issue. Per decision D-W, the wire body
// is the bare JSON string Username itself (e.g. "bob"), not an object -- Username is passed
// directly as PostJSON's body argument so json.Marshal serializes it as a literal JSON string, not
// {"username": "bob"}. It carries the same optional post-mutation read-back triad as
// AssignIssue/UpdateIssueComment.
type AddIssueWatcherInput struct {
	IssueIDOrKey string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	Username     string   `json:"username" jsonschema:"Username to add as a watcher (required)"`
	ReturnIssue  bool     `json:"returnIssue,omitempty" jsonschema:"Read the issue back after adding the watcher"`
	ReturnFields []string `json:"returnFields,omitempty" jsonschema:"Optional Jira fields query values for the read-back; requires returnIssue=true"`
	ReturnExpand []string `json:"returnExpand,omitempty" jsonschema:"Optional Jira expand query values for the read-back; requires returnIssue=true"`
}

// RemoveIssueWatcherInput removes one Jira user as a watcher from an issue. Per decision D-W,
// Username travels on the query string for this endpoint -- unlike AddIssueWatcherInput, there is no
// request body at all. It carries the same optional post-mutation read-back triad as
// AssignIssue/UpdateIssueComment.
type RemoveIssueWatcherInput struct {
	IssueIDOrKey string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	Username     string   `json:"username" jsonschema:"Username to remove as a watcher (required)"`
	ReturnIssue  bool     `json:"returnIssue,omitempty" jsonschema:"Read the issue back after removing the watcher"`
	ReturnFields []string `json:"returnFields,omitempty" jsonschema:"Optional Jira fields query values for the read-back; requires returnIssue=true"`
	ReturnExpand []string `json:"returnExpand,omitempty" jsonschema:"Optional Jira expand query values for the read-back; requires returnIssue=true"`
}

// VoteIssueInput adds the authenticated user's vote to a Jira issue. Jira's vote endpoint takes no
// request body at all (decision D-2). It carries the same optional post-mutation read-back triad as
// AssignIssue/UpdateIssueComment.
type VoteIssueInput struct {
	IssueIDOrKey string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	ReturnIssue  bool     `json:"returnIssue,omitempty" jsonschema:"Read the issue back after voting"`
	ReturnFields []string `json:"returnFields,omitempty" jsonschema:"Optional Jira fields query values for the read-back; requires returnIssue=true"`
	ReturnExpand []string `json:"returnExpand,omitempty" jsonschema:"Optional Jira expand query values for the read-back; requires returnIssue=true"`
}

// UnvoteIssueInput removes the authenticated user's vote from a Jira issue. Like VoteIssueInput,
// this endpoint takes no request body and no query parameters. It carries the same optional
// post-mutation read-back triad as AssignIssue/UpdateIssueComment.
type UnvoteIssueInput struct {
	IssueIDOrKey string   `json:"issueIdOrKey" jsonschema:"Jira issue ID or key"`
	ReturnIssue  bool     `json:"returnIssue,omitempty" jsonschema:"Read the issue back after unvoting"`
	ReturnFields []string `json:"returnFields,omitempty" jsonschema:"Optional Jira fields query values for the read-back; requires returnIssue=true"`
	ReturnExpand []string `json:"returnExpand,omitempty" jsonschema:"Optional Jira expand query values for the read-back; requires returnIssue=true"`
}

// CreateIssueLinkInput creates a native Jira issue link between two issues. Type, InwardIssue, and
// OutwardIssue are required native Jira JSON shapes (e.g. Type: {"name":"Blocks"}, InwardIssue:
// {"key":"PROJ-123"}). There is no post-mutation read-back triad here: the call links two separate
// issues, so there is no single unambiguous anchor to refresh (plan decision D-R's exclusion list).
type CreateIssueLinkInput struct {
	Type         map[string]any `json:"type" jsonschema:"Native Jira issue link type, e.g. {\"name\":\"Blocks\"} (required)"`
	InwardIssue  map[string]any `json:"inwardIssue" jsonschema:"Native Jira inward issue reference, e.g. {\"key\":\"PROJ-123\"} (required)"`
	OutwardIssue map[string]any `json:"outwardIssue" jsonschema:"Native Jira outward issue reference, e.g. {\"key\":\"PROJ-124\"} (required)"`
	Comment      map[string]any `json:"comment,omitempty" jsonschema:"Optional native Jira comment payload attached to the link"`
}

// CreateComponentInput creates a Jira project Component by project key. The Jira 6.4.14 endpoint
// wants a body field named "project", so ProjectKey is intentionally not forwarded under its MCP
// input name and no projectId variant is modeled. ProjectKey and Name are trimmed only for
// blank-value validation; nonblank caller-provided values are forwarded unchanged.
type CreateComponentInput struct {
	ProjectKey   string `json:"projectKey" jsonschema:"Jira project key that receives the Component (required); sent to Jira as body field project"`
	Name         string `json:"name" jsonschema:"Component name (required)"`
	Description  string `json:"description,omitempty" jsonschema:"Optional Component description"`
	LeadUserName string `json:"leadUserName,omitempty" jsonschema:"Optional Jira username for the Component lead"`
	AssigneeType string `json:"assigneeType,omitempty" jsonschema:"Optional Jira assignee type: PROJECT_DEFAULT, COMPONENT_LEAD, PROJECT_LEAD, or UNASSIGNED"`
}

// GetComponentInput selects exactly one Jira Component by its REST path ID.
type GetComponentInput struct {
	ComponentID string `json:"componentId" jsonschema:"Jira Component ID (required)"`
}

// UpdateComponentInput applies a partial Jira Component update. Pointer fields preserve the
// difference between an omitted field and an explicitly supplied empty string, which Jira can use to
// clear mutable Component fields.
type UpdateComponentInput struct {
	ComponentID  string  `json:"componentId" jsonschema:"Jira Component ID (required)"`
	Name         *string `json:"name,omitempty" jsonschema:"Optional replacement Component name"`
	Description  *string `json:"description,omitempty" jsonschema:"Optional replacement Component description; empty string is sent when explicitly supplied"`
	LeadUserName *string `json:"leadUserName,omitempty" jsonschema:"Optional replacement Component lead username; empty string is sent when explicitly supplied"`
	AssigneeType *string `json:"assigneeType,omitempty" jsonschema:"Optional Jira assignee type: PROJECT_DEFAULT, COMPONENT_LEAD, PROJECT_LEAD, or UNASSIGNED"`
}

// DeleteComponentInput deletes one Jira Component. MoveIssuesTo is optional and, when set, is sent
// only as Jira's query parameter; source/target project policy and same-target behavior remain Jira
// server decisions.
type DeleteComponentInput struct {
	ComponentID  string  `json:"componentId" jsonschema:"Jira Component ID to delete (required)"`
	MoveIssuesTo *string `json:"moveIssuesTo,omitempty" jsonschema:"Optional replacement Component ID for affected issues; sent as a query parameter"`
}

// GetComponentIssueCountInput selects one Component's Jira related issue count endpoint.
type GetComponentIssueCountInput struct {
	ComponentID string `json:"componentId" jsonschema:"Jira Component ID (required)"`
}

// ListProjectComponentsInput selects the Jira project whose bare Component array should be listed.
type ListProjectComponentsInput struct {
	ProjectIDOrKey string `json:"projectIdOrKey" jsonschema:"Jira project ID or key (required)"`
}

// Authenticate verifies candidate credentials with Jira before replacing the active process session.
// Explicit tool input wins; an empty field falls back to JIRA_USERNAME/JIRA_PASSWORD read at call time.
func (s *Service) Authenticate(ctx context.Context, input AuthenticateInput) result.Envelope {
	username := strings.TrimSpace(input.Username)
	password := input.Password
	if username == "" && s.getenv != nil {
		username = strings.TrimSpace(s.getenv("JIRA_USERNAME"))
	}
	if password == "" && s.getenv != nil {
		password = s.getenv("JIRA_PASSWORD")
	}
	if username == "" || password == "" {
		return result.Fail("jira", "jira_authenticate", "VALIDATION_ERROR", "username and password are required (pass them as tool input or set JIRA_USERNAME/JIRA_PASSWORD)")
	}
	candidate := auth.NewCredential(username, password)
	var serverInfo map[string]any
	if err := s.client.GetJSON(ctx, candidate, "/serverInfo", nil, &serverInfo); err != nil {
		return jiraClientError("jira_authenticate", "JIRA_AUTHENTICATION_FAILED", err)
	}
	var myself map[string]any
	if err := s.client.GetJSON(ctx, candidate, "/myself", nil, &myself); err != nil {
		return jiraClientError("jira_authenticate", "JIRA_AUTHENTICATION_FAILED", err)
	}
	s.store.Replace(candidate)
	return result.OK("jira", "jira_authenticate", map[string]any{
		"server": observability.Redact(serverInfo),
		"user":   observability.Redact(myself),
	})
}

// GetIssue returns Jira's original issue JSON under data.issue after session and path validation.
func (s *Service) GetIssue(ctx context.Context, input GetIssueInput) result.Envelope {
	cred, err := s.requireCredential("jira_get_issue")
	if err != nil {
		return *err
	}
	issueID, invalid := cleanIssueID("jira_get_issue", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	query := optionalQuery(input.Fields, input.Expand)
	var issue map[string]any
	if err := s.client.GetJSON(ctx, cred, "/issue/"+issueID, query, &issue); err != nil {
		return jiraClientError("jira_get_issue", "", err)
	}
	return result.OK("jira", "jira_get_issue", map[string]any{"issue": issue})
}

// AddIssueComment posts a single Jira comment and leaves role/group existence checks to Jira.
func (s *Service) AddIssueComment(ctx context.Context, input AddCommentInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_add_issue_comment")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_add_issue_comment", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if strings.TrimSpace(input.Body) == "" {
		return result.Fail("jira", "jira_add_issue_comment", "VALIDATION_ERROR", "body is required")
	}
	body := map[string]any{"body": input.Body}
	if input.Visibility != nil {
		visibilityType := strings.TrimSpace(input.Visibility.Type)
		visibilityValue := strings.TrimSpace(input.Visibility.Value)
		if visibilityValue == "" || (visibilityType != "role" && visibilityType != "group") {
			return result.Fail("jira", "jira_add_issue_comment", "VALIDATION_ERROR", "visibility requires type role or group and a value")
		}
		body["visibility"] = Visibility{Type: visibilityType, Value: visibilityValue}
	}
	var comment map[string]any
	if err := s.client.PostJSON(ctx, cred, "/issue/"+issueID+"/comment", body, &comment); err != nil {
		return jiraClientError("jira_add_issue_comment", "", err)
	}
	return result.OK("jira", "jira_add_issue_comment", comment)
}

// UpdateIssueFields sends native Jira fields/update JSON and optionally reads back the issue once.
func (s *Service) UpdateIssueFields(ctx context.Context, input UpdateIssueInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_update_issue_fields")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_update_issue_fields", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if len(input.Fields) == 0 && len(input.Update) == 0 {
		return result.Fail("jira", "jira_update_issue_fields", "VALIDATION_ERROR", "fields or update is required")
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_update_issue_fields", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	body := map[string]any{}
	if len(input.Fields) > 0 {
		body["fields"] = input.Fields
	}
	if len(input.Update) > 0 {
		body["update"] = input.Update
	}
	if err := s.client.PutJSON(ctx, cred, "/issue/"+issueID, body, nil); err != nil {
		return jiraClientError("jira_update_issue_fields", "", err)
	}
	return s.refreshAfterMutation(ctx, cred, "jira_update_issue_fields", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
}

// TransitionIssue executes exactly one Jira workflow transition by direct ID or exact name match.
func (s *Service) TransitionIssue(ctx context.Context, input TransitionIssueInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_transition_issue")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_transition_issue", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_transition_issue", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	transitionIDBlank := strings.TrimSpace(input.TransitionID) == ""
	transitionNameBlank := strings.TrimSpace(input.TransitionName) == ""
	if transitionIDBlank == transitionNameBlank {
		return result.Fail("jira", "jira_transition_issue", "VALIDATION_ERROR", "exactly one of transitionId or transitionName is required")
	}
	id := input.TransitionID
	if !transitionNameBlank {
		resolved, err := s.resolveTransitionName(ctx, cred, issueID, input.TransitionName)
		if err != nil {
			return *err
		}
		id = resolved
	}
	body := map[string]any{"transition": map[string]any{"id": id}}
	if len(input.Fields) > 0 {
		body["fields"] = input.Fields
	}
	if len(input.Update) > 0 {
		body["update"] = input.Update
	}
	if err := s.client.PostJSON(ctx, cred, "/issue/"+issueID+"/transitions", body, nil); err != nil {
		return jiraClientError("jira_transition_issue", "", err)
	}
	return s.refreshAfterMutation(ctx, cred, "jira_transition_issue", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
}

// CreateIssue creates exactly one new Jira issue from native fields/update JSON. There is no
// post-mutation read-back triad here (unlike the † tools): the call creates a brand-new issue, so
// there is no pre-existing issue to refresh (plan decision D-R's exclusion list).
func (s *Service) CreateIssue(ctx context.Context, input CreateIssueInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_create_issue")
	if errEnv != nil {
		return *errEnv
	}
	if len(input.Fields) == 0 {
		return result.Fail("jira", "jira_create_issue", "VALIDATION_ERROR", "fields is required")
	}
	body := map[string]any{"fields": input.Fields}
	if len(input.Update) > 0 {
		body["update"] = input.Update
	}
	var created map[string]any
	if err := s.client.PostJSON(ctx, cred, "/issue", body, &created); err != nil {
		return jiraClientError("jira_create_issue", "", err)
	}
	return result.OK("jira", "jira_create_issue", observability.Redact(created))
}

// BulkCreateIssues creates one or more Jira issues in a single upstream call. Per decision D-I, a
// non-empty upstream "errors" array in a 2xx response does NOT make this tool report failure: HTTP
// 2xx means the bulk request itself was accepted, and callers must inspect data.errors themselves to
// see which individual issueUpdates rows failed.
func (s *Service) BulkCreateIssues(ctx context.Context, input BulkCreateIssuesInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_bulk_create_issues")
	if errEnv != nil {
		return *errEnv
	}
	if len(input.IssueUpdates) == 0 {
		return result.Fail("jira", "jira_bulk_create_issues", "VALIDATION_ERROR", "issueUpdates is required and must not be empty")
	}
	body := map[string]any{"issueUpdates": input.IssueUpdates}
	var created map[string]any
	if err := s.client.PostJSON(ctx, cred, "/issue/bulk", body, &created); err != nil {
		return jiraClientError("jira_bulk_create_issues", "", err)
	}
	return result.OK("jira", "jira_bulk_create_issues", observability.Redact(created))
}

// DeleteIssue permanently deletes one Jira issue. There is no post-mutation read-back triad: the
// issue no longer exists after this call succeeds.
func (s *Service) DeleteIssue(ctx context.Context, input DeleteIssueInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_delete_issue")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_delete_issue", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	// DeleteSubtasks is a plain bool (not *bool): "true only when set" is exactly Go's zero-value
	// behavior here, so the deleteSubtasks query key is added only when the caller opted in.
	var q query
	if input.DeleteSubtasks {
		q = q.add("deleteSubtasks", "true")
	}
	if err := s.client.DeleteJSON(ctx, cred, "/issue/"+issueID, map[string][]string(q), nil); err != nil {
		return jiraClientError("jira_delete_issue", "", err)
	}
	return result.OK("jira", "jira_delete_issue", map[string]any{"mutationApplied": true})
}

// AssignIssue assigns or unassigns a Jira issue's assignee. Decisions D-A/D-H: an empty/omitted Name
// always sends the literal body {"name": ""} (never a null body) to trigger Jira's own
// config-dependent unassign behavior; Jira's separate "automatic assignee via null" state is
// deliberately not modeled as a distinct input here.
func (s *Service) AssignIssue(ctx context.Context, input AssignIssueInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_assign_issue")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_assign_issue", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_assign_issue", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	body := map[string]any{"name": input.Name}
	if err := s.client.PutJSON(ctx, cred, "/issue/"+issueID+"/assignee", body, nil); err != nil {
		return jiraClientError("jira_assign_issue", "", err)
	}
	return s.refreshAfterMutation(ctx, cred, "jira_assign_issue", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
}

// SearchIssues runs a JQL search via POST (decision D-4, avoiding GET URL-length limits) and builds
// its request body from only the fields the caller actually set, so unset optional parameters are
// never sent as explicit JSON nulls or empty arrays.
func (s *Service) SearchIssues(ctx context.Context, input SearchIssuesInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_search_issues")
	if errEnv != nil {
		return *errEnv
	}
	jql := strings.TrimSpace(input.JQL)
	if jql == "" {
		return result.Fail("jira", "jira_search_issues", "VALIDATION_ERROR", "jql is required")
	}
	body := map[string]any{"jql": jql}
	if input.StartAt != nil {
		body["startAt"] = *input.StartAt
	}
	if input.MaxResults != nil {
		body["maxResults"] = *input.MaxResults
	}
	if len(input.Fields) > 0 {
		body["fields"] = input.Fields
	}
	if len(input.Expand) > 0 {
		body["expand"] = input.Expand
	}
	if input.ValidateQuery != nil {
		body["validateQuery"] = *input.ValidateQuery
	}
	var searchResult map[string]any
	if err := s.client.PostJSON(ctx, cred, "/search", body, &searchResult); err != nil {
		return jiraClientError("jira_search_issues", "", err)
	}
	return result.OK("jira", "jira_search_issues", observability.Redact(searchResult))
}

// ListIssueComments returns Jira's native paginated comment listing for one issue. Read-only: no
// post-mutation read-back triad applies (there is no mutation to read back after).
func (s *Service) ListIssueComments(ctx context.Context, input ListIssueCommentsInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_list_issue_comments")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_list_issue_comments", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	// orderBy is added only when the caller sets it (query.add trims and omits blank values); this is
	// the risk-mitigation called out in the plan for Jira 6.4.14's patch-dependent orderBy support.
	var q query
	q = q.int("startAt", input.StartAt).int("maxResults", input.MaxResults).add("orderBy", input.OrderBy)
	if len(input.Expand) > 0 {
		q = q.add("expand", strings.Join(input.Expand, ","))
	}
	var comments map[string]any
	if err := s.client.GetJSON(ctx, cred, "/issue/"+issueID+"/comment", map[string][]string(q), &comments); err != nil {
		return jiraClientError("jira_list_issue_comments", "", err)
	}
	return result.OK("jira", "jira_list_issue_comments", observability.Redact(comments))
}

// UpdateIssueComment edits one existing Jira comment's body (and optionally its visibility), then
// optionally reads the parent issue back via the same triad/guard as AssignIssue. The updated comment
// is always returned under data.comment; when the read-back triggers, refreshAfterMutation's own
// mutationApplied/issue keys are merged alongside it rather than replacing it.
func (s *Service) UpdateIssueComment(ctx context.Context, input UpdateIssueCommentInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_update_issue_comment")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_update_issue_comment", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	commentID, invalidComment := cleanPathSegment("jira_update_issue_comment", "commentId", input.CommentID)
	if invalidComment != nil {
		return *invalidComment
	}
	if strings.TrimSpace(input.Body) == "" {
		return result.Fail("jira", "jira_update_issue_comment", "VALIDATION_ERROR", "body is required")
	}
	// Same guard as AssignIssue/UpdateIssueFields/TransitionIssue: refresh-only options are rejected
	// before any network call when the caller didn't actually request a refresh.
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_update_issue_comment", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	body := map[string]any{"body": input.Body}
	if input.Visibility != nil {
		visibilityType := strings.TrimSpace(input.Visibility.Type)
		visibilityValue := strings.TrimSpace(input.Visibility.Value)
		if visibilityValue == "" || (visibilityType != "role" && visibilityType != "group") {
			return result.Fail("jira", "jira_update_issue_comment", "VALIDATION_ERROR", "visibility requires type role or group and a value")
		}
		body["visibility"] = Visibility{Type: visibilityType, Value: visibilityValue}
	}
	var comment map[string]any
	if err := s.client.PutJSON(ctx, cred, "/issue/"+issueID+"/comment/"+commentID, body, &comment); err != nil {
		return jiraClientError("jira_update_issue_comment", "", err)
	}
	envelope := s.refreshAfterMutation(ctx, cred, "jira_update_issue_comment", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
	if data, ok := envelope.Data.(map[string]any); ok {
		data["comment"] = observability.Redact(comment)
		envelope.Data = data
	}
	return envelope
}

// DeleteIssueComment permanently deletes one Jira comment (204, no body), then optionally reads the
// parent issue back via the same triad/guard as AssignIssue. Unlike DeleteIssue, the parent issue
// still exists after this call, so the read-back triad applies here.
func (s *Service) DeleteIssueComment(ctx context.Context, input DeleteIssueCommentInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_delete_issue_comment")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_delete_issue_comment", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	commentID, invalidComment := cleanPathSegment("jira_delete_issue_comment", "commentId", input.CommentID)
	if invalidComment != nil {
		return *invalidComment
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_delete_issue_comment", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	if err := s.client.DeleteJSON(ctx, cred, "/issue/"+issueID+"/comment/"+commentID, nil, nil); err != nil {
		return jiraClientError("jira_delete_issue_comment", "", err)
	}
	return s.refreshAfterMutation(ctx, cred, "jira_delete_issue_comment", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
}

// ListIssueTransitions returns Jira's available transitions for one issue. Read-only: no
// post-mutation read-back triad applies here.
func (s *Service) ListIssueTransitions(ctx context.Context, input ListIssueTransitionsInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_list_issue_transitions")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_list_issue_transitions", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	var q query
	q = q.add("transitionId", input.TransitionID)
	if len(input.Expand) > 0 {
		// Caller-supplied Expand always replaces the default outright; it is never merged with it.
		q = q.add("expand", strings.Join(input.Expand, ","))
	} else {
		// Spec's own recommendation: default to transitions.fields so callers can see which fields a
		// transition screen requires without an extra round trip.
		q = q.add("expand", "transitions.fields")
	}
	var transitions map[string]any
	if err := s.client.GetJSON(ctx, cred, "/issue/"+issueID+"/transitions", map[string][]string(q), &transitions); err != nil {
		return jiraClientError("jira_list_issue_transitions", "", err)
	}
	return result.OK("jira", "jira_list_issue_transitions", observability.Redact(transitions))
}

// AddIssueAttachment uploads one file to a Jira issue via multipart/form-data. Jira 6.4.14 responds
// with HTTP 200 (not 201, unlike most other create-style endpoints here) and a JSON ARRAY of
// attachment objects rather than a single object, so the response is unmarshaled into []any and
// wrapped as {"attachments": [...]} for a stable, redactable envelope shape. The
// X-Atlassian-Token: nocheck header is mandatory: Jira 6.4.x otherwise rejects the upload as an XSRF
// violation (DoMultipart already applies it via extraHeaders).
func (s *Service) AddIssueAttachment(ctx context.Context, input AddIssueAttachmentInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_add_issue_attachment")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_add_issue_attachment", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if strings.TrimSpace(input.Filename) == "" {
		return result.Fail("jira", "jira_add_issue_attachment", "VALIDATION_ERROR", "filename is required")
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_add_issue_attachment", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	// MCP tool inputs are JSON-only (no raw file handle), so the caller sends the file content as
	// base64; a decode failure is a caller input error, not an upstream one, so it fails validation
	// before any network call.
	content, err := base64.StdEncoding.DecodeString(input.ContentBase64)
	if err != nil {
		return result.Fail("jira", "jira_add_issue_attachment", "VALIDATION_ERROR", "contentBase64 must be valid base64")
	}
	var attachments []any
	extraHeaders := map[string]string{"X-Atlassian-Token": "nocheck"}
	if err := s.client.DoMultipart(ctx, cred, "/issue/"+issueID+"/attachments", nil, "file", input.Filename, bytes.NewReader(content), extraHeaders, &attachments); err != nil {
		return jiraClientError("jira_add_issue_attachment", "", err)
	}
	envelope := s.refreshAfterMutation(ctx, cred, "jira_add_issue_attachment", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
	if data, ok := envelope.Data.(map[string]any); ok {
		data["attachments"] = observability.Redact(attachments)
		envelope.Data = data
	}
	return envelope
}

// DeleteIssueAttachment permanently deletes one Jira attachment (204, no body). The REST path root is
// /attachment/{id}, NOT /issue/{id}/attachment/{id} -- attachments have no issue-scoped path prefix, so
// no cleanIssueID validation applies here and no post-mutation read-back triad exists (there is no
// issue key in this tool's input at all).
func (s *Service) DeleteIssueAttachment(ctx context.Context, input DeleteIssueAttachmentInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_delete_issue_attachment")
	if errEnv != nil {
		return *errEnv
	}
	attachmentID, invalid := cleanPathSegment("jira_delete_issue_attachment", "attachmentId", input.AttachmentID)
	if invalid != nil {
		return *invalid
	}
	if err := s.client.DeleteJSON(ctx, cred, "/attachment/"+attachmentID, nil, nil); err != nil {
		return jiraClientError("jira_delete_issue_attachment", "", err)
	}
	return result.OK("jira", "jira_delete_issue_attachment", map[string]any{"mutationApplied": true})
}

// ListIssueWorklogs returns Jira's native paginated worklog listing for one issue. Read-only: no
// post-mutation read-back triad applies here.
func (s *Service) ListIssueWorklogs(ctx context.Context, input ListIssueWorklogsInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_list_issue_worklogs")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_list_issue_worklogs", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	var worklogs map[string]any
	if err := s.client.GetJSON(ctx, cred, "/issue/"+issueID+"/worklog", nil, &worklogs); err != nil {
		return jiraClientError("jira_list_issue_worklogs", "", err)
	}
	return result.OK("jira", "jira_list_issue_worklogs", observability.Redact(worklogs))
}

// AddIssueWorklog records a Jira worklog entry via a single POST that carries both a JSON body
// (comment/started/timeSpentSeconds) and query parameters (adjustEstimate/newEstimate/reduceBy),
// matching Jira's own request shape for this endpoint, then optionally reads the issue back via the
// same triad/guard as AssignIssue/UpdateIssueComment.
func (s *Service) AddIssueWorklog(ctx context.Context, input AddIssueWorklogInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_add_issue_worklog")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_add_issue_worklog", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if input.TimeSpentSeconds <= 0 {
		return result.Fail("jira", "jira_add_issue_worklog", "VALIDATION_ERROR", "timeSpentSeconds must be greater than zero")
	}
	// adjustEstimate, when set, must be exactly one of Jira's four accepted values; anything else is a
	// caller input error rather than something Jira itself should be left to reject.
	adjustEstimate := strings.TrimSpace(input.AdjustEstimate)
	if adjustEstimate != "" {
		switch adjustEstimate {
		case "new", "leave", "manual", "auto":
		default:
			return result.Fail("jira", "jira_add_issue_worklog", "VALIDATION_ERROR", "adjustEstimate must be one of new, leave, manual, auto")
		}
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_add_issue_worklog", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	body := map[string]any{"timeSpentSeconds": input.TimeSpentSeconds}
	if strings.TrimSpace(input.Comment) != "" {
		body["comment"] = input.Comment
	}
	if strings.TrimSpace(input.Started) != "" {
		body["started"] = input.Started
	}
	// newEstimate/reduceBy are only meaningful alongside a set adjustEstimate, so they are added to the
	// query only in that branch -- never unconditionally, and never in the JSON body.
	var q query
	if adjustEstimate != "" {
		q = q.add("adjustEstimate", adjustEstimate).add("newEstimate", input.NewEstimate).add("reduceBy", input.ReduceBy)
	}
	var worklog map[string]any
	if err := s.client.PostJSONQuery(ctx, cred, "/issue/"+issueID+"/worklog", map[string][]string(q), body, &worklog); err != nil {
		return jiraClientError("jira_add_issue_worklog", "", err)
	}
	envelope := s.refreshAfterMutation(ctx, cred, "jira_add_issue_worklog", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
	if data, ok := envelope.Data.(map[string]any); ok {
		data["worklog"] = observability.Redact(worklog)
		envelope.Data = data
	}
	return envelope
}

// GetIssueWatchers returns Jira's native watcher listing for one issue. Read-only: no post-mutation
// read-back triad applies here.
func (s *Service) GetIssueWatchers(ctx context.Context, input GetIssueWatchersInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_get_issue_watchers")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_get_issue_watchers", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	var watchers map[string]any
	if err := s.client.GetJSON(ctx, cred, "/issue/"+issueID+"/watchers", nil, &watchers); err != nil {
		return jiraClientError("jira_get_issue_watchers", "", err)
	}
	return result.OK("jira", "jira_get_issue_watchers", observability.Redact(watchers))
}

// AddIssueWatcher adds one user as a watcher on a Jira issue (204, no response body). Per decision
// D-W, the request body is the bare JSON string input.Username itself (e.g. "bob") -- Username is
// passed directly as PostJSON's body argument, so json.Marshal serializes it to a literal JSON
// string rather than an object. It then optionally reads the issue back via the same triad/guard as
// AssignIssue/UpdateIssueComment.
func (s *Service) AddIssueWatcher(ctx context.Context, input AddIssueWatcherInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_add_issue_watcher")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_add_issue_watcher", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if strings.TrimSpace(input.Username) == "" {
		return result.Fail("jira", "jira_add_issue_watcher", "VALIDATION_ERROR", "username is required")
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_add_issue_watcher", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	if err := s.client.PostJSON(ctx, cred, "/issue/"+issueID+"/watchers", input.Username, nil); err != nil {
		return jiraClientError("jira_add_issue_watcher", "", err)
	}
	return s.refreshAfterMutation(ctx, cred, "jira_add_issue_watcher", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
}

// RemoveIssueWatcher removes one user as a watcher from a Jira issue (204, no response body). Per
// decision D-W, username travels on the query string for this endpoint -- unlike AddIssueWatcher's
// bare-string POST body, there is no request body here at all. It then optionally reads the issue
// back via the same triad/guard as AssignIssue/UpdateIssueComment.
func (s *Service) RemoveIssueWatcher(ctx context.Context, input RemoveIssueWatcherInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_remove_issue_watcher")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_remove_issue_watcher", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if strings.TrimSpace(input.Username) == "" {
		return result.Fail("jira", "jira_remove_issue_watcher", "VALIDATION_ERROR", "username is required")
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_remove_issue_watcher", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	var q query
	q = q.add("username", input.Username)
	if err := s.client.DeleteJSON(ctx, cred, "/issue/"+issueID+"/watchers", map[string][]string(q), nil); err != nil {
		return jiraClientError("jira_remove_issue_watcher", "", err)
	}
	return s.refreshAfterMutation(ctx, cred, "jira_remove_issue_watcher", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
}

// VoteIssue adds the authenticated user's vote to a Jira issue via a bodyless POST (204, no
// response body), then optionally reads the issue back via the same triad/guard as
// AssignIssue/UpdateIssueComment.
func (s *Service) VoteIssue(ctx context.Context, input VoteIssueInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_vote_issue")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_vote_issue", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_vote_issue", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	if err := s.client.PostJSON(ctx, cred, "/issue/"+issueID+"/votes", nil, nil); err != nil {
		return jiraClientError("jira_vote_issue", "", err)
	}
	return s.refreshAfterMutation(ctx, cred, "jira_vote_issue", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
}

// UnvoteIssue removes the authenticated user's vote from a Jira issue (204, no response body, no
// query parameters), then optionally reads the issue back via the same triad/guard as
// AssignIssue/UpdateIssueComment.
func (s *Service) UnvoteIssue(ctx context.Context, input UnvoteIssueInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_unvote_issue")
	if errEnv != nil {
		return *errEnv
	}
	issueID, invalid := cleanIssueID("jira_unvote_issue", input.IssueIDOrKey)
	if invalid != nil {
		return *invalid
	}
	if !input.ReturnIssue && (len(input.ReturnFields) > 0 || len(input.ReturnExpand) > 0) {
		return result.Fail("jira", "jira_unvote_issue", "VALIDATION_ERROR", "returnFields and returnExpand require returnIssue=true")
	}
	if err := s.client.DeleteJSON(ctx, cred, "/issue/"+issueID+"/votes", nil, nil); err != nil {
		return jiraClientError("jira_unvote_issue", "", err)
	}
	return s.refreshAfterMutation(ctx, cred, "jira_unvote_issue", issueID, input.ReturnIssue, input.ReturnFields, input.ReturnExpand)
}

// CreateIssueLink creates a Jira issue link between two issues via POST /issueLink -- a path outside
// the /issue/... tree, so cleanIssueID does not apply to any of its inputs (Type/InwardIssue/
// OutwardIssue are native Jira JSON objects, not URL path segments). Jira typically returns 201 with
// no response body, so success is reported as {mutationApplied:true} rather than echoing an upstream
// object. There is no post-mutation read-back triad (plan decision D-R): two issues are involved and
// there is no single unambiguous anchor to refresh.
func (s *Service) CreateIssueLink(ctx context.Context, input CreateIssueLinkInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_create_issue_link")
	if errEnv != nil {
		return *errEnv
	}
	if len(input.Type) == 0 || len(input.InwardIssue) == 0 || len(input.OutwardIssue) == 0 {
		return result.Fail("jira", "jira_create_issue_link", "VALIDATION_ERROR", "type, inwardIssue, and outwardIssue are required")
	}
	body := map[string]any{
		"type":         input.Type,
		"inwardIssue":  input.InwardIssue,
		"outwardIssue": input.OutwardIssue,
	}
	if len(input.Comment) > 0 {
		body["comment"] = input.Comment
	}
	if err := s.client.PostJSON(ctx, cred, "/issueLink", body, nil); err != nil {
		return jiraClientError("jira_create_issue_link", "", err)
	}
	return result.OK("jira", "jira_create_issue_link", map[string]any{"mutationApplied": true})
}

// CreateComponent posts one Jira Component create request. Only MCP-owned required-field validation
// happens locally; duplicate names, lead validity, and assignee configuration are Jira policy and are
// passed through via jiraClientError.
func (s *Service) CreateComponent(ctx context.Context, input CreateComponentInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_create_component")
	if errEnv != nil {
		return *errEnv
	}
	if strings.TrimSpace(input.ProjectKey) == "" {
		return result.Fail("jira", "jira_create_component", "VALIDATION_ERROR", "projectKey is required")
	}
	if strings.TrimSpace(input.Name) == "" {
		return result.Fail("jira", "jira_create_component", "VALIDATION_ERROR", "name is required")
	}
	body := map[string]any{"project": input.ProjectKey, "name": input.Name}
	if input.Description != "" {
		body["description"] = input.Description
	}
	if input.LeadUserName != "" {
		body["leadUserName"] = input.LeadUserName
	}
	if input.AssigneeType != "" {
		body["assigneeType"] = input.AssigneeType
	}
	var component map[string]any
	if err := s.client.PostJSON(ctx, cred, "/component", body, &component); err != nil {
		return jiraClientError("jira_create_component", "", err)
	}
	return result.OK("jira", "jira_create_component", observability.Redact(component))
}

// GetComponent returns Jira's native Component JSON for one safe Component ID.
func (s *Service) GetComponent(ctx context.Context, input GetComponentInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_get_component")
	if errEnv != nil {
		return *errEnv
	}
	componentID, invalid := cleanPathSegment("jira_get_component", "componentId", input.ComponentID)
	if invalid != nil {
		return *invalid
	}
	var component map[string]any
	if err := s.client.GetJSON(ctx, cred, "/component/"+componentID, nil, &component); err != nil {
		return jiraClientError("jira_get_component", "", err)
	}
	return result.OK("jira", "jira_get_component", observability.Redact(component))
}

// UpdateComponent sends only caller-supplied Component fields. A successful empty 200 response maps
// to the shared mutation acknowledgement; a non-empty JSON 200 is returned as Jira sent it.
func (s *Service) UpdateComponent(ctx context.Context, input UpdateComponentInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_update_component")
	if errEnv != nil {
		return *errEnv
	}
	componentID, invalid := cleanPathSegment("jira_update_component", "componentId", input.ComponentID)
	if invalid != nil {
		return *invalid
	}
	body := map[string]any{}
	if input.Name != nil {
		body["name"] = *input.Name
	}
	if input.Description != nil {
		body["description"] = *input.Description
	}
	if input.LeadUserName != nil {
		body["leadUserName"] = *input.LeadUserName
	}
	if input.AssigneeType != nil {
		body["assigneeType"] = *input.AssigneeType
	}
	if len(body) == 0 {
		return result.Fail("jira", "jira_update_component", "VALIDATION_ERROR", "at least one component field is required")
	}
	var component map[string]any
	if err := s.client.PutJSON(ctx, cred, "/component/"+componentID, body, &component); err != nil {
		return jiraClientError("jira_update_component", "", err)
	}
	if component == nil {
		return result.OK("jira", "jira_update_component", map[string]any{"mutationApplied": true})
	}
	return result.OK("jira", "jira_update_component", observability.Redact(component))
}

// DeleteComponent sends exactly one Jira DELETE request. It deliberately performs no source/target
// lookup, project comparison, retry, or same-component prevalidation; Jira owns those policies.
func (s *Service) DeleteComponent(ctx context.Context, input DeleteComponentInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_delete_component")
	if errEnv != nil {
		return *errEnv
	}
	componentID, invalid := cleanPathSegment("jira_delete_component", "componentId", input.ComponentID)
	if invalid != nil {
		return *invalid
	}
	var q query
	if input.MoveIssuesTo != nil {
		moveIssuesTo, invalid := cleanPathSegment("jira_delete_component", "moveIssuesTo", *input.MoveIssuesTo)
		if invalid != nil {
			return *invalid
		}
		q = q.add("moveIssuesTo", moveIssuesTo)
	}
	if err := s.client.DeleteJSON(ctx, cred, "/component/"+componentID, map[string][]string(q), nil); err != nil {
		return jiraClientError("jira_delete_component", "", err)
	}
	return result.OK("jira", "jira_delete_component", map[string]any{"mutationApplied": true})
}

// GetComponentIssueCount returns Jira's native relatedIssueCounts object for one Component.
func (s *Service) GetComponentIssueCount(ctx context.Context, input GetComponentIssueCountInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_get_component_issue_count")
	if errEnv != nil {
		return *errEnv
	}
	componentID, invalid := cleanPathSegment("jira_get_component_issue_count", "componentId", input.ComponentID)
	if invalid != nil {
		return *invalid
	}
	var counts map[string]any
	if err := s.client.GetJSON(ctx, cred, "/component/"+componentID+"/relatedIssueCounts", nil, &counts); err != nil {
		return jiraClientError("jira_get_component_issue_count", "", err)
	}
	return result.OK("jira", "jira_get_component_issue_count", observability.Redact(counts))
}

// ListProjectComponents returns Jira's bare Component array for a project ID or key without adding
// local pagination, uniqueness, or assignee policy.
func (s *Service) ListProjectComponents(ctx context.Context, input ListProjectComponentsInput) result.Envelope {
	cred, errEnv := s.requireCredential("jira_list_project_components")
	if errEnv != nil {
		return *errEnv
	}
	projectIDOrKey, invalid := cleanPathSegment("jira_list_project_components", "projectIdOrKey", input.ProjectIDOrKey)
	if invalid != nil {
		return *invalid
	}
	var components []any
	if err := s.client.GetJSON(ctx, cred, "/project/"+projectIDOrKey+"/components", nil, &components); err != nil {
		return jiraClientError("jira_list_project_components", "", err)
	}
	return result.OK("jira", "jira_list_project_components", observability.Redact(components))
}

// requireCredential blocks business tools before jira_authenticate and deliberately sends no network request.
func (s *Service) requireCredential(tool string) (auth.Credential, *result.Envelope) {
	cred, err := s.store.Snapshot()
	if errors.Is(err, auth.ErrNotAuthenticated) {
		env := result.Fail("jira", tool, "JIRA_NOT_AUTHENTICATED", "Call jira_authenticate before using Jira issue tools.")
		return auth.Credential{}, &env
	}
	if err != nil {
		env := result.Fail("jira", tool, "JIRA_NOT_AUTHENTICATED", "Jira session is not available.")
		return auth.Credential{}, &env
	}
	return cred, nil
}

// refreshAfterMutation reports the write as applied even when the optional read-back fails.
func (s *Service) refreshAfterMutation(ctx context.Context, cred auth.Credential, tool, issue string, refresh bool, fields, expand []string) result.Envelope {
	if !refresh {
		return result.OK("jira", tool, map[string]any{"mutationApplied": true})
	}
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/issue/"+issue, optionalQuery(fields, expand), &out); err != nil {
		return result.OK("jira", tool, map[string]any{
			"mutationApplied": true,
			"issue":           nil,
			"refreshError":    result.Error{Code: "JIRA_REFRESH_FAILED", Message: "Issue updated, but refreshing the issue failed."},
		})
	}
	return result.OK("jira", tool, map[string]any{"mutationApplied": true, "issue": out})
}

// resolveTransitionName refuses to guess when Jira returns zero or multiple exact name matches.
func (s *Service) resolveTransitionName(ctx context.Context, cred auth.Credential, issue, name string) (string, *result.Envelope) {
	var payload struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	if err := s.client.GetJSON(ctx, cred, "/issue/"+issue+"/transitions", nil, &payload); err != nil {
		env := jiraClientError("jira_transition_issue", "", err)
		return "", &env
	}
	var matches []string
	for _, transition := range payload.Transitions {
		if transition.Name == name {
			matches = append(matches, transition.ID)
		}
	}
	if len(matches) == 0 {
		env := result.Fail("jira", "jira_transition_issue", "JIRA_TRANSITION_NOT_FOUND", fmt.Sprintf("transition %q was not found", name))
		return "", &env
	}
	if len(matches) > 1 {
		env := result.Fail("jira", "jira_transition_issue", "JIRA_TRANSITION_AMBIGUOUS", fmt.Sprintf("transition %q matched more than once", name))
		return "", &env
	}
	return matches[0], nil
}

// cleanIssueID keeps issue identifiers to one URL path segment before the client builds a request URL.
// It is a thin, message-compatible wrapper over cleanPathSegment so existing error text and tests
// for jira_get_issue and friends are unchanged by the generalization below.
func cleanIssueID(tool, value string) (string, *result.Envelope) {
	return cleanPathSegment(tool, "issueIdOrKey", value)
}

// cleanPathSegment validates that value is non-blank and safe to embed as exactly one URL path
// segment (no "/", "?", "#", or "\\"), naming the offending input via field in the returned
// VALIDATION_ERROR message. Tools that validate other single-segment identifiers — commentId,
// attachmentId, worklog id, etc. — call this directly so the error names the right field.
func cleanPathSegment(tool, field, value string) (string, *result.Envelope) {
	id := strings.TrimSpace(value)
	if id == "" {
		env := result.Fail("jira", tool, "VALIDATION_ERROR", field+" is required")
		return "", &env
	}
	if strings.ContainsAny(id, "/?#\\") {
		env := result.Fail("jira", tool, "VALIDATION_ERROR", field+" must be one URL path segment")
		return "", &env
	}
	return id, nil
}

// query builds a Jira REST query-string map fluently. Each method is nil-safe (a nil query can be
// extended in place) and only adds its key when the underlying value is actually set, so handlers
// can chain optional parameters without per-tool boilerplate or accidentally sending zero-value
// placeholders for parameters the caller never supplied. Ported from
// internal/bitbucket/tools/service.go's identical helper (same method names/semantics). Consumed by
// jira_delete_issue (deleteSubtasks), jira_list_issue_comments (startAt/maxResults/orderBy),
// jira_list_issue_transitions (transitionId/expand), jira_add_issue_worklog
// (adjustEstimate/newEstimate/reduceBy), and jira_remove_issue_watcher (username) today;
// jira_search_issues builds a JSON body via PostJSON instead (its
// startAt/maxResults/fields/expand/validateQuery are POST body fields, not query params, per the
// confirmed Group B spec).
type query map[string][]string

// add sets k to v when v is non-blank after trimming; a blank v omits the key entirely.
func (q query) add(k, v string) query {
	if q == nil {
		q = query{}
	}
	if strings.TrimSpace(v) != "" {
		q[k] = []string{v}
	}
	return q
}

// bool sets k to v's string form only when v is non-nil, so "unset" (nil) and "explicitly false"
// remain distinguishable in the resulting query.
func (q query) bool(k string, v *bool) query {
	if q == nil {
		q = query{}
	}
	if v != nil {
		q[k] = []string{strconv.FormatBool(*v)}
	}
	return q
}

// int sets k to v's string form only when v is non-nil, so "unset" (nil) and "explicitly zero"
// remain distinguishable in the resulting query.
func (q query) int(k string, v *int) query {
	if q == nil {
		q = query{}
	}
	if v != nil {
		q[k] = []string{strconv.Itoa(*v)}
	}
	return q
}

// optionalQuery passes Jira fields/expand arrays as comma-joined native query values.
func optionalQuery(fields, expand []string) map[string][]string {
	query := map[string][]string{}
	if len(fields) > 0 {
		query["fields"] = []string{strings.Join(fields, ",")}
	}
	if len(expand) > 0 {
		query["expand"] = []string{strings.Join(expand, ",")}
	}
	if len(query) == 0 {
		return nil
	}
	return query
}

// jiraClientError maps sanitized Jira client failures into the shared result envelope.
func jiraClientError(tool, fallback string, err error) result.Envelope {
	var httpErr *client.HTTPError
	if errors.As(err, &httpErr) {
		code := httpErr.Code
		if fallback != "" {
			code = fallback
		}
		return result.FailHTTPDetail("jira", tool, code, httpErr.Message, httpErr.StatusCode, httpErr.Detail)
	}
	code := "UPSTREAM_UNREACHABLE"
	if fallback != "" {
		code = fallback
	}
	return result.Fail("jira", tool, code, "Jira request failed")
}
