package tools

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	bbclient "github.com/chiendao1808/atlassian-mcp/internal/bitbucket/client"
	"github.com/chiendao1808/atlassian-mcp/internal/result"
)

// Service owns the Bitbucket tool handlers and shared client configuration.
type Service struct {
	client   *bbclient.Client
	userSlug string
}

// NewService binds Bitbucket REST access and the configured reviewer identity.
func NewService(client *bbclient.Client, userSlug string) *Service {
	return &Service{client: client, userSlug: userSlug}
}

func (s *Service) getJSON(ctx context.Context, tool, slug, path string, q map[string][]string, key string) result.Envelope {
	ep, env := s.endpoint(tool, slug, path)
	if env != nil {
		return *env
	}
	var out map[string]any
	if err := s.client.DoJSON(ctx, http.MethodGet, ep, q, nil, &out); err != nil {
		return clientError(tool, err)
	}
	return result.OK("bitbucket", tool, map[string]any{key: out, "repositorySlug": slug})
}

func (s *Service) getJSONSegments(ctx context.Context, tool, slug string, segments []string, q map[string][]string, key string) result.Envelope {
	ep, env := s.endpointSegments(tool, slug, segments...)
	if env != nil {
		return *env
	}
	var out map[string]any
	if err := s.client.DoJSON(ctx, http.MethodGet, ep, q, nil, &out); err != nil {
		return clientError(tool, err)
	}
	return result.OK("bitbucket", tool, map[string]any{key: out, "repositorySlug": slug})
}

func (s *Service) postJSON(ctx context.Context, tool, slug, path string, q map[string][]string, body any, key string) result.Envelope {
	ep, env := s.endpoint(tool, slug, path)
	if env != nil {
		return *env
	}
	var out map[string]any
	if err := s.client.DoJSON(ctx, http.MethodPost, ep, q, body, &out); err != nil {
		return clientError(tool, err)
	}
	return result.OK("bitbucket", tool, map[string]any{key: out, "repositorySlug": slug})
}

func (s *Service) putJSON(ctx context.Context, tool, slug, path string, q map[string][]string, body any, key string) result.Envelope {
	ep, env := s.endpoint(tool, slug, path)
	if env != nil {
		return *env
	}
	var out map[string]any
	if err := s.client.DoJSON(ctx, http.MethodPut, ep, q, body, &out); err != nil {
		return clientError(tool, err)
	}
	return result.OK("bitbucket", tool, map[string]any{key: out, "repositorySlug": slug})
}

func (s *Service) diff(ctx context.Context, tool, slug, base, path string, q map[string][]string) result.Envelope {
	return s.diffSegments(ctx, tool, slug, strings.Split(base, "/"), path, q)
}

func (s *Service) diffSegments(ctx context.Context, tool, slug string, baseSegments []string, path string, q map[string][]string) result.Envelope {
	var ep bbclient.Endpoint
	var err error
	if path == "" {
		ep, err = s.client.RepositoryEndpoint(slug, "/projects/{projectKey}/repos/{repositorySlug}/"+strings.Join(baseSegments, "/"), baseSegments...)
	} else {
		// Dynamic refs can contain slashes; keep API segments separate from the optional file path.
		ep, err = s.client.RepositoryNestedFileEndpoint(slug, "/projects/{projectKey}/repos/{repositorySlug}/"+strings.Join(baseSegments, "/")+"/{path}", baseSegments, path)
	}
	if err != nil {
		return fail(tool, "repositorySlug and path are required")
	}
	var out map[string]any
	if err := s.client.DoJSON(ctx, http.MethodGet, ep, q, nil, &out); err != nil {
		return clientError(tool, err)
	}
	return result.OK("bitbucket", tool, map[string]any{"diff": out, "repositorySlug": slug})
}

func (s *Service) endpoint(tool, slug, path string) (bbclient.Endpoint, *result.Envelope) {
	if strings.TrimSpace(slug) == "" {
		env := fail(tool, "repositorySlug is required")
		return bbclient.Endpoint{}, &env
	}
	var segments []string
	if path != "" {
		segments = strings.Split(path, "/")
	}
	ep, err := s.client.RepositoryEndpoint(slug, "/projects/{projectKey}/repos/{repositorySlug}/"+path, segments...)
	if err != nil {
		env := fail(tool, "repositorySlug is required")
		return bbclient.Endpoint{}, &env
	}
	return ep, nil
}

func (s *Service) endpointSegments(tool, slug string, segments ...string) (bbclient.Endpoint, *result.Envelope) {
	if strings.TrimSpace(slug) == "" {
		env := fail(tool, "repositorySlug is required")
		return bbclient.Endpoint{}, &env
	}
	ep, err := s.client.RepositoryEndpoint(slug, "/projects/{projectKey}/repos/{repositorySlug}/"+strings.Join(segments, "/"), segments...)
	if err != nil {
		env := fail(tool, "repositorySlug is required")
		return bbclient.Endpoint{}, &env
	}
	return ep, nil
}

type query map[string][]string

func (q query) add(k, v string) query {
	if q == nil {
		q = query{}
	}
	if strings.TrimSpace(v) != "" {
		q[k] = []string{v}
	}
	return q
}

func (q query) bool(k string, v *bool) query {
	if q == nil {
		q = query{}
	}
	if v != nil {
		q[k] = []string{strconv.FormatBool(*v)}
	}
	return q
}

func (q query) int(k string, v *int) query {
	if q == nil {
		q = query{}
	}
	if v != nil {
		q[k] = []string{strconv.Itoa(*v)}
	}
	return q
}

func (q query) page(start, limit *int) query {
	return q.int("start", start).int("limit", limit)
}

// q is nil-safe so handlers can fluently add optional parameters without boilerplate.
func q(kv ...string) query {
	q := query{}
	for i := 0; i+1 < len(kv); i += 2 {
		q.add(kv[i], kv[i+1])
	}
	if len(q) == 0 {
		return nil
	}
	return q
}

func fail(tool, msg string) result.Envelope {
	return result.Fail("bitbucket", tool, "VALIDATION_ERROR", msg)
}

func clientError(tool string, err error) result.Envelope {
	var httpErr *bbclient.HTTPError
	if errors.As(err, &httpErr) {
		code := httpErr.Code
		if tool == "bitbucket_commit_file" && httpErr.StatusCode == http.StatusConflict {
			code = "BITBUCKET_COMMIT_FILE_CONFLICT"
		}
		return result.FailHTTPDetail("bitbucket", tool, code, httpErr.Message, httpErr.StatusCode, httpErr.Detail)
	}
	return result.Fail("bitbucket", tool, "UPSTREAM_UNREACHABLE", fmt.Sprintf("Bitbucket request failed: %v", err))
}
