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

// validatedPage enforces the common Confluence collection paging contract.
func validatedPage(tool string, start, limit *int, defaultLimit int) (int, *result.Envelope) {
	if start != nil && *start < 0 {
		env := result.Fail("confluence", tool, "VALIDATION_ERROR", "start must be non-negative")
		return 0, &env
	}
	if limit != nil && *limit <= 0 {
		env := result.Fail("confluence", tool, "VALIDATION_ERROR", "limit must be positive")
		return 0, &env
	}
	if limit != nil {
		return *limit, nil
	}
	return defaultLimit, nil
}

// cleanPathSegment keeps caller-supplied IDs and keys to exactly one URL segment.
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
