package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/confluence/client"
	"github.com/chiendao1808/atlassian-mcp/internal/observability"
	"github.com/chiendao1808/atlassian-mcp/internal/result"
)

// Service owns Confluence tool handlers and the process-local authenticated session.
type Service struct {
	client *client.Client
	store  *auth.SessionStore
	getenv func(string) string
}

// NewService binds Confluence REST access, session storage, and env fallback lookup.
func NewService(client *client.Client, store *auth.SessionStore, getenv func(string) string) *Service {
	return &Service{client: client, store: store, getenv: getenv}
}

// Store exposes the session store for module wiring and focused tests.
func (s *Service) Store() *auth.SessionStore { return s.store }

// AuthenticateInput carries the Confluence credential payload accepted by the toolset.
type AuthenticateInput struct {
	Username string `json:"username,omitempty" jsonschema:"Confluence username; falls back to CONFLUENCE_USERNAME if omitted"`
	Password string `json:"password,omitempty" jsonschema:"Sensitive Confluence password; falls back to CONFLUENCE_PASSWORD if omitted"`
}

// SearchContentInput selects Confluence content with raw CQL and optional passthrough expansions.
type SearchContentInput struct {
	CQL        string `json:"cql" jsonschema:"Raw Confluence CQL query (required)"`
	CQLContext string `json:"cqlcontext,omitempty" jsonschema:"Optional Confluence cqlcontext query value"`
	Expand     string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value"`
	Start      *int   `json:"start,omitempty" jsonschema:"Optional non-negative start offset"`
	Limit      *int   `json:"limit,omitempty" jsonschema:"Optional positive limit; defaults to 25 when omitted"`
}

// GetContentInput selects one Confluence content item and optional native query parameters.
type GetContentInput struct {
	ContentID string `json:"contentId" jsonschema:"Confluence content ID (required)"`
	Status    string `json:"status,omitempty" jsonschema:"Optional Confluence content status"`
	Version   *int   `json:"version,omitempty" jsonschema:"Optional positive content version"`
	Expand    string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value, e.g. body.storage,body.view"`
}

// Authenticate verifies candidate credentials with /user/current before replacing the session.
func (s *Service) Authenticate(ctx context.Context, input AuthenticateInput) result.Envelope {
	return s.authenticate(ctx, input, func(candidate auth.Credential) bool {
		s.store.Replace(candidate)
		return true
	})
}

// AuthenticateIfSessionUnchanged validates credentials but commits them only if no newer session exists.
func (s *Service) AuthenticateIfSessionUnchanged(ctx context.Context, input AuthenticateInput, expected *auth.Credential) result.Envelope {
	return s.authenticate(ctx, input, func(candidate auth.Credential) bool {
		return s.store.ReplaceIfUnchanged(candidate, expected)
	})
}

func (s *Service) authenticate(ctx context.Context, input AuthenticateInput, commit func(auth.Credential) bool) result.Envelope {
	username := strings.TrimSpace(input.Username)
	password := input.Password
	if username == "" && s.getenv != nil {
		username = strings.TrimSpace(s.getenv("CONFLUENCE_USERNAME"))
	}
	if password == "" && s.getenv != nil {
		password = s.getenv("CONFLUENCE_PASSWORD")
	}
	if username == "" || password == "" {
		return result.Fail("confluence", "confluence_authenticate", "VALIDATION_ERROR", "username and password are required (pass them as tool input or set CONFLUENCE_USERNAME/CONFLUENCE_PASSWORD)")
	}
	candidate := auth.NewCredential(username, password)
	var user map[string]any
	if err := s.client.GetJSON(ctx, candidate, "/user/current", nil, &user); err != nil {
		return confluenceClientError("confluence_authenticate", "CONFLUENCE_AUTHENTICATION_FAILED", err)
	}
	if user["type"] != "known" {
		return result.Fail("confluence", "confluence_authenticate", "CONFLUENCE_AUTHENTICATION_FAILED", "Confluence did not return a known authenticated user")
	}
	commit(candidate)
	return result.OK("confluence", "confluence_authenticate", map[string]any{"user": observability.Redact(user)})
}

// SearchContent runs a raw CQL content search after authentication.
func (s *Service) SearchContent(ctx context.Context, input SearchContentInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_search_content")
	if errEnv != nil {
		return *errEnv
	}
	if strings.TrimSpace(input.CQL) == "" {
		return result.Fail("confluence", "confluence_search_content", "VALIDATION_ERROR", "cql is required")
	}
	if input.Start != nil && *input.Start < 0 {
		return result.Fail("confluence", "confluence_search_content", "VALIDATION_ERROR", "start must be non-negative")
	}
	if input.Limit != nil && *input.Limit <= 0 {
		return result.Fail("confluence", "confluence_search_content", "VALIDATION_ERROR", "limit must be positive")
	}
	limit := 25
	if input.Limit != nil {
		limit = *input.Limit
	}
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/content/search", q("cql", input.CQL, "cqlcontext", input.CQLContext, "expand", input.Expand).int("start", input.Start).intValue("limit", limit), &out); err != nil {
		return confluenceClientError("confluence_search_content", "", err)
	}
	return result.OK("confluence", "confluence_search_content", observability.Redact(out))
}

// GetContent retrieves one Confluence content item by ID after authentication.
func (s *Service) GetContent(ctx context.Context, input GetContentInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_get_content")
	if errEnv != nil {
		return *errEnv
	}
	contentID, invalid := cleanPathSegment("confluence_get_content", "contentId", input.ContentID)
	if invalid != nil {
		return *invalid
	}
	if input.Version != nil && *input.Version <= 0 {
		return result.Fail("confluence", "confluence_get_content", "VALIDATION_ERROR", "version must be positive")
	}
	query := q("status", input.Status, "expand", input.Expand)
	query = query.int("version", input.Version)
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/content/"+contentID, query, &out); err != nil {
		return confluenceClientError("confluence_get_content", "", err)
	}
	return result.OK("confluence", "confluence_get_content", observability.Redact(out))
}

// requireCredential blocks read tools before confluence_authenticate and sends no network request.
func (s *Service) requireCredential(tool string) (auth.Credential, *result.Envelope) {
	cred, err := s.store.Snapshot()
	if errors.Is(err, auth.ErrNotAuthenticated) {
		env := result.Fail("confluence", tool, "CONFLUENCE_NOT_AUTHENTICATED", "Call confluence_authenticate before using Confluence tools.")
		return auth.Credential{}, &env
	}
	if err != nil {
		env := result.Fail("confluence", tool, "CONFLUENCE_NOT_AUTHENTICATED", "Confluence session is not available.")
		return auth.Credential{}, &env
	}
	return cred, nil
}

func cleanPathSegment(tool, field, value string) (string, *result.Envelope) {
	id := strings.TrimSpace(value)
	if id == "" {
		env := result.Fail("confluence", tool, "VALIDATION_ERROR", field+" is required")
		return "", &env
	}
	if strings.ContainsAny(id, "/?#\\") {
		env := result.Fail("confluence", tool, "VALIDATION_ERROR", field+" must be one URL path segment")
		return "", &env
	}
	return id, nil
}

type query map[string][]string

func q(kv ...string) query {
	out := query{}
	for i := 0; i+1 < len(kv); i += 2 {
		if strings.TrimSpace(kv[i+1]) != "" {
			out[kv[i]] = []string{kv[i+1]}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (q query) int(k string, v *int) query {
	if v == nil {
		return q
	}
	return q.intValue(k, *v)
}

func (q query) intValue(k string, v int) query {
	if q == nil {
		q = query{}
	}
	q[k] = []string{strconv.Itoa(v)}
	return q
}

func confluenceClientError(tool, fallback string, err error) result.Envelope {
	var httpErr *client.HTTPError
	if errors.As(err, &httpErr) {
		code := httpErr.Code
		if fallback != "" {
			code = fallback
		}
		return result.FailHTTPDetail("confluence", tool, code, httpErr.Message, httpErr.StatusCode, httpErr.Detail)
	}
	code := "UPSTREAM_UNREACHABLE"
	if fallback != "" {
		code = fallback
	}
	return result.Fail("confluence", tool, code, fmt.Sprintf("Confluence request failed: %v", err))
}
