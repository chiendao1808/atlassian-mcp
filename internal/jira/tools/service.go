package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/jira/auth"
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
func cleanIssueID(tool, value string) (string, *result.Envelope) {
	id := strings.TrimSpace(value)
	if id == "" {
		env := result.Fail("jira", tool, "VALIDATION_ERROR", "issueIdOrKey is required")
		return "", &env
	}
	if strings.ContainsAny(id, "/?#\\") {
		env := result.Fail("jira", tool, "VALIDATION_ERROR", "issueIdOrKey must be one URL path segment")
		return "", &env
	}
	return id, nil
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
