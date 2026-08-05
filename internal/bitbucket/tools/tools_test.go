package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	bbclient "github.com/chiendao1808/atlassian-mcp/internal/bitbucket/client"
)

func newTestService(baseURL string, hc *http.Client) *Service {
	return NewService(bbclient.New(baseURL, "PRJ", "token", hc, 1<<20, bbclient.Options{}), "svc-user")
}

func TestDefinitionsExposeExactlyTheBitbucketToolSet(t *testing.T) {
	want := []string{
		"bitbucket_get_repository",
		"bitbucket_list_branches",
		"bitbucket_get_default_branch",
		"bitbucket_create_branch",
		"bitbucket_get_file",
		"bitbucket_list_commits",
		"bitbucket_get_commit",
		"bitbucket_get_commit_changes",
		"bitbucket_get_commit_diff",
		"bitbucket_compare_commits",
		"bitbucket_compare_changes",
		"bitbucket_compare_diff",
		"bitbucket_commit_file",
		"bitbucket_list_pull_requests",
		"bitbucket_get_pull_request",
		"bitbucket_get_pull_request_activities",
		"bitbucket_get_pull_request_commits",
		"bitbucket_get_pull_request_changes",
		"bitbucket_get_pull_request_diff",
		"bitbucket_check_pull_request_mergeability",
		"bitbucket_create_pull_request",
		"bitbucket_add_pull_request_comment",
		"bitbucket_set_pull_request_review_status",
		"bitbucket_merge_pull_request",
		"bitbucket_decline_pull_request",
		"bitbucket_reopen_pull_request",
		"bitbucket_update_pull_request",
	}
	defs := Definitions()
	var got []string
	for _, def := range defs {
		got = append(got, def.Name)
		if def.Annotations == nil || def.Annotations.OpenWorldHint == nil || !*def.Annotations.OpenWorldHint {
			t.Fatalf("%s missing open-world annotation", def.Name)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tool names\n got: %v\nwant: %v", got, want)
	}
	if !defs[0].Annotations.ReadOnlyHint || defs[3].Annotations.ReadOnlyHint || defs[12].Annotations.DestructiveHint == nil || !*defs[12].Annotations.DestructiveHint {
		t.Fatalf("unexpected annotations")
	}
	if defs[26].Annotations.DestructiveHint == nil || *defs[26].Annotations.DestructiveHint {
		t.Fatalf("bitbucket_update_pull_request should be additive (destructiveHint=false)")
	}
}

func TestRepositoryAndBranchEndpoints(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Method + " " + r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			raw += "?" + r.URL.RawQuery
		}
		requests = append(requests, raw)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL+"/bitbucket", server.Client())
	if out := svc.GetRepository(context.Background(), repositoryInput{RepositorySlug: "repo slug"}); !out.Success {
		t.Fatalf("repo out=%+v", out)
	}
	details := true
	if out := svc.ListBranches(context.Background(), listBranchesInput{RepositorySlug: "repo", FilterText: "feat", Details: &details, Limit: intPtr(25)}); !out.Success {
		t.Fatalf("branches out=%+v", out)
	}
	want0 := "GET /bitbucket/rest/api/1.0/projects/PRJ/repos/repo%20slug"
	if requests[0] != want0 {
		t.Fatalf("request 0 = %s, want %s", requests[0], want0)
	}
	if !strings.HasPrefix(requests[1], "GET /bitbucket/rest/api/1.0/projects/PRJ/repos/repo/branches?") || !strings.Contains(requests[1], "filterText=feat") || !strings.Contains(requests[1], "details=true") || !strings.Contains(requests[1], "limit=25") {
		t.Fatalf("branch request = %s", requests[1])
	}
}

func TestNestedDiffPathAndPullRequestTransitionVersion(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.String())
		switch {
		case strings.Contains(r.URL.Path, "/pull-requests/7") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 7, "version": 11})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	if out := svc.GetCommitDiff(context.Background(), diffInput{RepositorySlug: "repo", CommitID: "abc/def", Path: "src/main.go"}); !out.Success {
		t.Fatalf("diff out=%+v", out)
	}
	noPrecheck := false
	if out := svc.MergePullRequest(context.Background(), transitionInput{RepositorySlug: "repo", PullRequestID: 7, Precheck: &noPrecheck}); !out.Success {
		t.Fatalf("merge out=%+v", out)
	}
	if requests[0] != "GET /rest/api/1.0/projects/PRJ/repos/repo/commits/abc%2Fdef/diff/src/main.go" {
		t.Fatalf("diff request = %s", requests[0])
	}
	if requests[1] != "GET /rest/api/1.0/projects/PRJ/repos/repo/pull-requests/7" || requests[2] != "POST /rest/api/1.0/projects/PRJ/repos/repo/pull-requests/7/merge?version=11" {
		t.Fatalf("transition requests = %v", requests)
	}
}

func TestCreatePullRequestIncludesProjectKeyInRefs(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	out := svc.CreatePullRequest(context.Background(), createPRInput{
		RepositorySlug: "repo",
		Title:          "title",
		FromBranch:     "feature",
		ToBranch:       "main",
	})
	if !out.Success {
		t.Fatalf("create out=%+v", out)
	}

	fromRef := body["fromRef"].(map[string]any)
	fromRepo := fromRef["repository"].(map[string]any)
	if _, ok := fromRepo["project"]; !ok {
		t.Fatalf("fromRef.repository missing project key: %+v", fromRepo)
	}
	toRef := body["toRef"].(map[string]any)
	toRepo := toRef["repository"].(map[string]any)
	toProject, ok := toRepo["project"].(map[string]any)
	if !ok {
		t.Fatalf("toRef.repository missing project key: %+v", toRepo)
	}
	if toProject["key"] != "PRJ" {
		t.Fatalf("toRef.repository.project.key = %v, want PRJ", toProject["key"])
	}
}

func intPtr(v int) *int { return &v }

func strPtr(v string) *string { return &v }

func TestUpdatePullRequestAutoPreservesOmittedFields(t *testing.T) {
	var requests []string
	var putBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.String())
		switch {
		case strings.Contains(r.URL.Path, "/pull-requests/9") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          9,
				"version":     4,
				"title":       "old",
				"description": "olddesc",
				"reviewers": []map[string]any{
					{
						"user": map[string]any{
							"name":        "charlie",
							"slug":        "charlie",
							"displayName": "Charlie C",
						},
						"role":               "REVIEWER",
						"approved":           true,
						"lastReviewedCommit": "abc123",
					},
				},
			})
		case strings.Contains(r.URL.Path, "/pull-requests/9") && r.Method == http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 9})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	out := svc.UpdatePullRequest(context.Background(), updatePRInput{
		RepositorySlug: "repo",
		PullRequestID:  9,
		Title:          strPtr("new"),
	})
	if !out.Success {
		t.Fatalf("update out=%+v", out)
	}

	if len(requests) != 2 || requests[0] != "GET /rest/api/1.0/projects/PRJ/repos/repo/pull-requests/9" || requests[1] != "PUT /rest/api/1.0/projects/PRJ/repos/repo/pull-requests/9" {
		t.Fatalf("requests = %v", requests)
	}
	if putBody["version"].(float64) != 4 {
		t.Fatalf("version = %v, want 4", putBody["version"])
	}
	if putBody["title"] != "new" {
		t.Fatalf("title = %v, want new (override)", putBody["title"])
	}
	if putBody["description"] != "olddesc" {
		t.Fatalf("description = %v, want olddesc (preserved)", putBody["description"])
	}
	reviewers, ok := putBody["reviewers"].([]any)
	if !ok || len(reviewers) != 1 {
		t.Fatalf("reviewers = %v, want one normalized entry", putBody["reviewers"])
	}
	reviewer := reviewers[0].(map[string]any)
	if len(reviewer) != 1 {
		t.Fatalf("reviewer = %v, want only the user key", reviewer)
	}
	user := reviewer["user"].(map[string]any)
	if len(user) != 1 || user["name"] != "charlie" {
		t.Fatalf("reviewer user = %v, want {name: charlie} only (no slug/displayName)", user)
	}
}

func TestNormalizeReviewersStripsReadOnlyFieldsAndPreservesUnnormalizable(t *testing.T) {
	full := []any{
		map[string]any{
			// A cleanly-normalizable participant: reduced to {"user":{"name":...}},
			// dropping read-only fields (slug, displayName, role, approved, ...).
			"user": map[string]any{
				"name":        "charlie",
				"slug":        "charlie",
				"displayName": "Charlie C",
			},
			"role":               "REVIEWER",
			"approved":           true,
			"lastReviewedCommit": "abc123",
		},
		map[string]any{
			// Empty user.name can't be normalized to a usable write entry, but
			// this is the "reviewers untouched" path: the reviewer must be
			// preserved (with its original user sub-object) rather than
			// silently dropped from the update.
			"user": map[string]any{"name": "", "slug": "empty-name-user"},
		},
		map[string]any{
			// Missing user.name entirely: same fallback-preserve behavior.
			"user": map[string]any{"slug": "no-name-user"},
		},
		map[string]any{
			// user.name of the wrong type: same fallback-preserve behavior.
			"user": map[string]any{"name": 42, "slug": "wrong-type-user"},
		},
		map[string]any{
			// No "user" key at all: not a participant object, nothing to
			// preserve, so this entry is skipped entirely.
			"role": "REVIEWER",
		},
	}
	got := normalizeReviewers(full)
	if len(got) != 4 {
		t.Fatalf("normalizeReviewers(full) returned %d entries, want 4 (the no-user entry skipped); got %+v", len(got), got)
	}

	// Entry 0: cleanly normalized down to {"user":{"name":"charlie"}} only.
	if user, ok := got[0]["user"].(map[string]any); !ok || len(user) != 1 || user["name"] != "charlie" || len(got[0]) != 1 {
		t.Fatalf("got[0] = %+v, want {user:{name:charlie}} only", got[0])
	}

	// Entry 1: empty name preserved verbatim (including the read-only slug),
	// not dropped and not reduced to {"name":""}.
	if user, ok := got[1]["user"].(map[string]any); !ok || user["name"] != "" || user["slug"] != "empty-name-user" {
		t.Fatalf("got[1] = %+v, want original user sub-object preserved verbatim", got[1])
	}

	// Entry 2: missing name preserved verbatim.
	if user, ok := got[2]["user"].(map[string]any); !ok || user["slug"] != "no-name-user" {
		t.Fatalf("got[2] = %+v, want original user sub-object preserved verbatim", got[2])
	}

	// Entry 3: wrong-type name preserved verbatim.
	if user, ok := got[3]["user"].(map[string]any); !ok || user["name"] != 42 || user["slug"] != "wrong-type-user" {
		t.Fatalf("got[3] = %+v, want original user sub-object preserved verbatim", got[3])
	}

	if got := normalizeReviewers(nil); got == nil || len(got) != 0 {
		t.Fatalf("normalizeReviewers(nil) = %v, want empty non-nil slice", got)
	}
	if got := normalizeReviewers("not-a-list"); got == nil || len(got) != 0 {
		t.Fatalf("normalizeReviewers(non-array) = %v, want empty non-nil slice", got)
	}
}

func TestUpdatePullRequestClearsReviewersWithEmptySlice(t *testing.T) {
	var putBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pull-requests/9") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 9, "version": 4, "title": "old"})
		case strings.Contains(r.URL.Path, "/pull-requests/9") && r.Method == http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 9})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	out := svc.UpdatePullRequest(context.Background(), updatePRInput{
		RepositorySlug: "repo",
		PullRequestID:  9,
		Reviewers:      &[]reviewerInput{},
	})
	if !out.Success {
		t.Fatalf("update out=%+v", out)
	}
	reviewers, ok := putBody["reviewers"]
	if !ok {
		t.Fatalf("reviewers key missing from PUT body, want present-but-empty")
	}
	list, ok := reviewers.([]any)
	if !ok || len(list) != 0 {
		t.Fatalf("reviewers = %v, want empty array", reviewers)
	}
}

func TestUpdatePullRequestPropagatesGetFailure(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.String())
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{{"message": "not found", "exceptionName": "NoSuchPullRequestException"}},
		})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	out := svc.UpdatePullRequest(context.Background(), updatePRInput{RepositorySlug: "repo", PullRequestID: 9})
	if out.Success {
		t.Fatalf("expected failure, got %+v", out)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %v, want exactly one GET (no PUT)", requests)
	}
}

func TestUpdatePullRequestPropagatesConflictWithoutRetry(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.String())
		switch {
		case strings.Contains(r.URL.Path, "/pull-requests/9") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 9, "version": 4, "title": "old"})
		case strings.Contains(r.URL.Path, "/pull-requests/9") && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]any{{"message": "stale version", "exceptionName": "PullRequestOutOfDateException"}},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	out := svc.UpdatePullRequest(context.Background(), updatePRInput{RepositorySlug: "repo", PullRequestID: 9, Title: strPtr("new")})
	if out.Success {
		t.Fatalf("expected failure, got %+v", out)
	}
	if out.Error == nil || out.Error.HTTPCode != http.StatusConflict {
		t.Fatalf("expected HTTP status 409 surfaced, got %+v", out.Error)
	}
	puts := 0
	for _, r := range requests {
		if strings.HasPrefix(r, "PUT ") {
			puts++
		}
	}
	if puts != 1 {
		t.Fatalf("PUT count = %d, want exactly 1 (no retry); requests=%v", puts, requests)
	}
}
