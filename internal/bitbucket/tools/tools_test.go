package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	bbclient "github.com/chiendao1808/atlassian-mcp/internal/bitbucket/client"
	"github.com/chiendao1808/atlassian-mcp/internal/result"
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

// newRecordingServer returns an httptest server that logs "METHOD path?query"
// lines and responds 200 {} for every request.
func newRecordingServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Method + " " + r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			raw += "?" + r.URL.RawQuery
		}
		requests = append(requests, raw)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

// newNoRequestServer returns an httptest server that fails the test if any
// request arrives.
func newNoRequestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return server
}

func assertValidation(t *testing.T, env result.Envelope, substr string) {
	t.Helper()
	if env.Success {
		t.Fatalf("expected failure, got success: %+v", env)
	}
	if env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %+v", env.Error)
	}
	if substr != "" && !strings.Contains(env.Error.Message, substr) {
		t.Fatalf("error message %q does not contain %q", env.Error.Message, substr)
	}
}

// VP-001: Cross-repo compare serializes fromRepo with configured project key.
func TestCompareToolsSerializeFromRepo(t *testing.T) {
	server, requests := newRecordingServer(t)
	svc := newTestService(server.URL, server.Client())
	ctx := context.Background()
	in := compareInput{RepositorySlug: "repo", From: "feature", To: "master", FromRepositorySlug: "other"}

	for _, call := range []struct {
		name string
		fn   func() result.Envelope
	}{
		{"commits", func() result.Envelope { return svc.CompareCommits(ctx, in) }},
		{"changes", func() result.Envelope { return svc.CompareChanges(ctx, in) }},
		{"diff", func() result.Envelope { return svc.CompareDiff(ctx, compareInput{RepositorySlug: "repo", From: "feature", To: "master", FromRepositorySlug: "other", Path: "f.txt"}) }},
	} {
		if out := call.fn(); !out.Success {
			t.Fatalf("%s: %+v", call.name, out)
		}
	}
	if len(*requests) != 3 {
		t.Fatalf("requests = %v, want 3", *requests)
	}
	for i, req := range *requests {
		if !strings.Contains(req, "fromRepo=PRJ%2Fother") {
			t.Fatalf("request[%d] = %s, want fromRepo=PRJ%%2Fother", i, req)
		}
		if strings.Contains(req, "fromRepositorySlug") {
			t.Fatalf("request[%d] = %s, must not contain fromRepositorySlug", i, req)
		}
		if !strings.Contains(req, "from=feature") || !strings.Contains(req, "to=master") {
			t.Fatalf("request[%d] = %s, missing from/to", i, req)
		}
	}
}

// VP-002: Same-repo compare omits fromRepo entirely.
func TestCompareSameRepoOmitsFromRepo(t *testing.T) {
	server, requests := newRecordingServer(t)
	svc := newTestService(server.URL, server.Client())
	out := svc.CompareCommits(context.Background(), compareInput{RepositorySlug: "repo", From: "a", To: "b"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if len(*requests) != 1 {
		t.Fatalf("requests = %v, want 1", *requests)
	}
	req := (*requests)[0]
	if strings.Contains(req, "fromRepo") || strings.Contains(req, "fromRepositorySlug") {
		t.Fatalf("request = %s, must not contain fromRepo or fromRepositorySlug", req)
	}
	if !strings.Contains(req, "from=a") || !strings.Contains(req, "to=b") {
		t.Fatalf("request = %s, missing from/to", req)
	}
}

// VP-003: Project/URL injection via fromRepositorySlug is rejected before any request.
func TestCompareRejectsSlashInFromRepositorySlug(t *testing.T) {
	server := newNoRequestServer(t)
	svc := newTestService(server.URL, server.Client())
	ctx := context.Background()
	in := compareInput{RepositorySlug: "repo", From: "a", To: "b", FromRepositorySlug: "OTHER/repo"}
	for _, call := range []struct {
		name string
		fn   func() result.Envelope
	}{
		{"commits", func() result.Envelope { return svc.CompareCommits(ctx, in) }},
		{"changes", func() result.Envelope { return svc.CompareChanges(ctx, in) }},
		{"diff", func() result.Envelope { return svc.CompareDiff(ctx, in) }},
	} {
		assertValidation(t, call.fn(), "fromRepositorySlug")
	}
}

// VP-004: commit_file update mode requires sourceCommitId.
func TestCommitFileUpdateRequiresSourceCommitId(t *testing.T) {
	server := newNoRequestServer(t)
	svc := newTestService(server.URL, server.Client())
	out := svc.CommitFile(context.Background(), commitFileInput{
		RepositorySlug: "repo", Path: "a.txt", Branch: "main", Mode: "update", Content: "x", Message: "m",
	})
	assertValidation(t, out, "sourceCommitId is required in update mode")
}

// VP-005: commit_file create mode rejects sourceCommitId and omits it from multipart.
func TestCommitFileCreateModeRejectsAndOmitsSourceCommitId(t *testing.T) {
	// Call 1: create with sourceCommitId → validation failure, no request.
	noReq := newNoRequestServer(t)
	svcNoReq := newTestService(noReq.URL, noReq.Client())
	out := svcNoReq.CommitFile(context.Background(), commitFileInput{
		RepositorySlug: "repo", Path: "a.txt", Branch: "main", Mode: "create", Content: "x", Message: "m", SourceCommitID: "abc",
	})
	assertValidation(t, out, "sourceCommitId must be omitted in create mode")

	// Call 2: create without sourceCommitId → success, one PUT, no sourceCommitId field.
	var requests []string
	var formValues map[string]string
	var contentBytes []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method)
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("multipart parse: %v", err)
		}
		formValues = map[string]string{}
		for k := range r.MultipartForm.Value {
			formValues[k] = r.FormValue(k)
		}
		file, _, err := r.FormFile("content")
		if err != nil {
			t.Errorf("content part: %v", err)
		} else {
			contentBytes, _ = io.ReadAll(file)
			_ = file.Close()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc"})
	}))
	t.Cleanup(server.Close)
	svc := newTestService(server.URL, server.Client())
	out = svc.CommitFile(context.Background(), commitFileInput{
		RepositorySlug: "repo", Path: "a.txt", Branch: "main", Mode: "create", Content: "hello", Message: "m",
	})
	if !out.Success {
		t.Fatalf("create out=%+v", out)
	}
	if len(requests) != 1 || requests[0] != http.MethodPut {
		t.Fatalf("requests = %v, want exactly one PUT", requests)
	}
	if formValues["branch"] != "main" || formValues["message"] != "m" {
		t.Fatalf("form values = %v", formValues)
	}
	if _, ok := formValues["sourceCommitId"]; ok {
		t.Fatalf("sourceCommitId must not be present in create mode, got %v", formValues)
	}
	if string(contentBytes) != "hello" {
		t.Fatalf("content part = %q, want exact bytes hello", contentBytes)
	}
}

// VP-006: commit_file update mode sends sourceCommitId in multipart, one PUT.
func TestCommitFileUpdateSendsSourceCommitId(t *testing.T) {
	var requests []string
	var formValues map[string]string
	var contentBytes []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method)
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("multipart parse: %v", err)
		}
		formValues = map[string]string{}
		for k := range r.MultipartForm.Value {
			formValues[k] = r.FormValue(k)
		}
		file, _, err := r.FormFile("content")
		if err != nil {
			t.Errorf("content part: %v", err)
		} else {
			contentBytes, _ = io.ReadAll(file)
			_ = file.Close()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc"})
	}))
	t.Cleanup(server.Close)
	svc := newTestService(server.URL, server.Client())
	out := svc.CommitFile(context.Background(), commitFileInput{
		RepositorySlug: "repo", Path: "a.txt", Branch: "main", Mode: "update", Content: "x", Message: "m",
		SourceCommitID: "abc0", SourceBranch: "base",
	})
	if !out.Success {
		t.Fatalf("update out=%+v", out)
	}
	if len(requests) != 1 || requests[0] != http.MethodPut {
		t.Fatalf("requests = %v, want exactly one PUT", requests)
	}
	if formValues["branch"] != "main" || formValues["message"] != "m" {
		t.Fatalf("form values = %v", formValues)
	}
	if formValues["sourceCommitId"] != "abc0" {
		t.Fatalf("sourceCommitId = %q, want abc0", formValues["sourceCommitId"])
	}
	if formValues["sourceBranch"] != "base" {
		t.Fatalf("sourceBranch = %q, want base", formValues["sourceBranch"])
	}
	if string(contentBytes) != "x" {
		t.Fatalf("content part = %q, want exact bytes x", contentBytes)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["singleFileCommit"] != true {
		t.Fatalf("data = %v, want singleFileCommit=true", out.Data)
	}
}

// VP-007: commit_file rejects missing or invalid mode.
func TestCommitFileModeValidation(t *testing.T) {
	server := newNoRequestServer(t)
	svc := newTestService(server.URL, server.Client())
	base := commitFileInput{RepositorySlug: "repo", Path: "a.txt", Branch: "main", Content: "x", Message: "m"}

	// Empty mode → validation error.
	in := base
	assertValidation(t, svc.CommitFile(context.Background(), in), "mode must be create or update")

	// "delete" → validation error.
	in = base
	in.Mode = "delete"
	assertValidation(t, svc.CommitFile(context.Background(), in), "mode must be create or update")

	// "UPDATE " (normalized) → proceeds to sourceCommitId validation.
	in = base
	in.Mode = "UPDATE "
	assertValidation(t, svc.CommitFile(context.Background(), in), "sourceCommitId is required in update mode")
}

// VP-008: commit_file 409 maps to BITBUCKET_COMMIT_FILE_CONFLICT with detail preserved, no retry.
func TestCommitFileConflictMapping(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method)
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{{"message": "stale source commit", "exceptionName": "SomeException"}},
		})
	}))
	t.Cleanup(server.Close)
	svc := newTestService(server.URL, server.Client())
	out := svc.CommitFile(context.Background(), commitFileInput{
		RepositorySlug: "repo", Path: "a.txt", Branch: "main", Mode: "update", Content: "x", Message: "m", SourceCommitID: "abc0",
	})
	if out.Success {
		t.Fatalf("expected failure, got %+v", out)
	}
	if out.Error == nil || out.Error.Code != "BITBUCKET_COMMIT_FILE_CONFLICT" {
		t.Fatalf("error code = %+v, want BITBUCKET_COMMIT_FILE_CONFLICT", out.Error)
	}
	if out.Error.HTTPCode != http.StatusConflict {
		t.Fatalf("HTTPCode = %d, want 409", out.Error.HTTPCode)
	}
	detail, ok := out.Error.Detail.([]map[string]any)
	if !ok || len(detail) != 1 || detail[0]["message"] != "stale source commit" {
		t.Fatalf("Detail = %v, want sanitized errors array with message preserved", out.Error.Detail)
	}
	puts := 0
	for _, m := range requests {
		if m == http.MethodPut {
			puts++
		}
	}
	if puts != 1 {
		t.Fatalf("PUT count = %d, want exactly 1 (no retry)", puts)
	}
}

// VP-009: sourceBranch is passed through when supplied (OQ-2 option i).
func TestCommitFileSourceBranchPassThrough(t *testing.T) {
	var formValues map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("multipart parse: %v", err)
		}
		formValues = map[string]string{}
		for k := range r.MultipartForm.Value {
			formValues[k] = r.FormValue(k)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc"})
	}))
	t.Cleanup(server.Close)
	svc := newTestService(server.URL, server.Client())
	out := svc.CommitFile(context.Background(), commitFileInput{
		RepositorySlug: "repo", Path: "a.txt", Branch: "new-branch", Mode: "create", Content: "x", Message: "m", SourceBranch: "main",
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if formValues["sourceBranch"] != "main" {
		t.Fatalf("sourceBranch = %q, want main", formValues["sourceBranch"])
	}
}

// VP-010: review status PUT body contains user.name, status, and correct approved mapping.
func TestReviewStatusBodyIncludesUser(t *testing.T) {
	var bodies []map[string]any
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.EscapedPath())
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	t.Cleanup(server.Close)
	svc := newTestService(server.URL, server.Client())
	ctx := context.Background()

	cases := []struct {
		input    string
		status   string
		approved bool
	}{
		{"approved", "APPROVED", true},
		{"NEEDS_WORK", "NEEDS_WORK", false},
		{"UNAPPROVED", "UNAPPROVED", false},
	}
	for _, tc := range cases {
		out := svc.SetPullRequestReviewStatus(ctx, reviewStatusInput{RepositorySlug: "repo", PullRequestID: 5, Status: tc.input})
		if !out.Success {
			t.Fatalf("status %s: %+v", tc.input, out)
		}
	}
	if len(bodies) != 3 {
		t.Fatalf("bodies = %d, want 3", len(bodies))
	}
	for i, tc := range cases {
		wantPath := "PUT /rest/api/1.0/projects/PRJ/repos/repo/pull-requests/5/participants/svc-user"
		if paths[i] != wantPath {
			t.Fatalf("path[%d] = %s, want %s", i, paths[i], wantPath)
		}
		body := bodies[i]
		user, ok := body["user"].(map[string]any)
		if !ok || user["name"] != "svc-user" {
			t.Fatalf("body[%d].user = %v, want {name: svc-user}", i, body["user"])
		}
		if body["status"] != tc.status {
			t.Fatalf("body[%d].status = %v, want %s", i, body["status"], tc.status)
		}
		if body["approved"] != tc.approved {
			t.Fatalf("body[%d].approved = %v, want %v", i, body["approved"], tc.approved)
		}
	}
}

// VP-011: review status without configured identity fails before any request.
func TestReviewStatusRequiresIdentity(t *testing.T) {
	server := newNoRequestServer(t)
	svc := NewService(bbclient.New(server.URL, "PRJ", "token", server.Client(), 1<<20, bbclient.Options{}), "")
	out := svc.SetPullRequestReviewStatus(context.Background(), reviewStatusInput{RepositorySlug: "repo", PullRequestID: 5, Status: "APPROVED"})
	if out.Success {
		t.Fatalf("expected failure, got %+v", out)
	}
	if out.Error == nil || out.Error.Code != "BITBUCKET_REVIEW_IDENTITY_REQUIRED" {
		t.Fatalf("error = %+v, want BITBUCKET_REVIEW_IDENTITY_REQUIRED", out.Error)
	}
}

// VP-013: P2-1..P2-3 commit read tools serialize newly documented params.
func TestCommitReadToolsSerializeDocumentedParams(t *testing.T) {
	server, requests := newRecordingServer(t)
	svc := newTestService(server.URL, server.Client())
	ctx := context.Background()
	tru := true

	// P2-1: ListCommits with ignoreMissing.
	if out := svc.ListCommits(ctx, commitListInput{RepositorySlug: "repo", IgnoreMissing: &tru}); !out.Success {
		t.Fatalf("ListCommits: %+v", out)
	}
	// P2-2: GetCommitChanges with since + withComments (no start).
	if out := svc.GetCommitChanges(ctx, commitPagedInput{RepositorySlug: "repo", CommitID: "abc", Since: "def", WithComments: &tru}); !out.Success {
		t.Fatalf("GetCommitChanges: %+v", out)
	}
	// P2-3: GetCommitDiff with since + withComments + autoSrcPath.
	if out := svc.GetCommitDiff(ctx, diffInput{RepositorySlug: "repo", CommitID: "abc", Since: "def", WithComments: &tru, AutoSrcPath: &tru}); !out.Success {
		t.Fatalf("GetCommitDiff: %+v", out)
	}

	if len(*requests) != 3 {
		t.Fatalf("requests = %v, want 3", *requests)
	}
	// P2-1: ignoreMissing=true present.
	if !strings.Contains((*requests)[0], "ignoreMissing=true") {
		t.Fatalf("ListCommits request = %s, want ignoreMissing=true", (*requests)[0])
	}
	// P2-2: since and withComments present, no start param.
	changesReq := (*requests)[1]
	if !strings.Contains(changesReq, "since=def") || !strings.Contains(changesReq, "withComments=true") {
		t.Fatalf("GetCommitChanges request = %s, missing since/withComments", changesReq)
	}
	if strings.Contains(changesReq, "start=") {
		t.Fatalf("GetCommitChanges request = %s, must not contain start", changesReq)
	}
	// P2-3: since, withComments, autoSrcPath present.
	diffReq := (*requests)[2]
	for _, param := range []string{"since=def", "withComments=true", "autoSrcPath=true"} {
		if !strings.Contains(diffReq, param) {
			t.Fatalf("GetCommitDiff request = %s, missing %s", diffReq, param)
		}
	}

	// Malformed inputs fail validation with zero additional requests.
	assertValidation(t, svc.ListCommits(ctx, commitListInput{RepositorySlug: "repo", Merges: "bogus"}), "merges")
	assertValidation(t, svc.GetCommitChanges(ctx, commitPagedInput{RepositorySlug: "repo"}), "commitId")
	assertValidation(t, svc.GetCommitDiff(ctx, diffInput{RepositorySlug: "repo"}), "commitId")
	if len(*requests) != 3 {
		t.Fatalf("requests after validation failures = %v, want still 3", *requests)
	}
}

// VP-014: P2-4 PR list participant filters serialize as continuous username.N/role.N/approved.N.
func TestListPullRequestsParticipantFilters(t *testing.T) {
	server, requests := newRecordingServer(t)
	svc := newTestService(server.URL, server.Client())
	ctx := context.Background()
	tru := true

	// Valid call: two filters, withAttributes, withProperties.
	out := svc.ListPullRequests(ctx, prListInput{
		RepositorySlug: "repo",
		Participants: []prParticipantFilter{
			{Username: "alice", Role: "REVIEWER", Approved: &tru},
			{Username: "bob"},
		},
		WithAttributes: &tru,
		WithProperties: &tru,
	})
	if !out.Success {
		t.Fatalf("valid call: %+v", out)
	}
	req := (*requests)[0]
	for _, param := range []string{"username.1=alice", "role.1=REVIEWER", "approved.1=true", "username.2=bob", "withAttributes=true", "withProperties=true"} {
		if !strings.Contains(req, param) {
			t.Fatalf("request = %s, missing %s", req, param)
		}
	}
	if strings.Contains(req, "role.2") || strings.Contains(req, "approved.2") {
		t.Fatalf("request = %s, must not contain role.2 or approved.2 (no gaps)", req)
	}
	if strings.Contains(req, "participant=") {
		t.Fatalf("request = %s, must not contain legacy participant= param", req)
	}

	// 11 filters → validation error, no request.
	noReq := newNoRequestServer(t)
	svcNoReq := newTestService(noReq.URL, noReq.Client())
	eleven := make([]prParticipantFilter, 11)
	for i := range eleven {
		eleven[i] = prParticipantFilter{Username: "u"}
	}
	assertValidation(t, svcNoReq.ListPullRequests(ctx, prListInput{RepositorySlug: "repo", Participants: eleven}), "at most 10")

	// Invalid enums → validation error, no request.
	for _, tc := range []struct {
		name string
		in   prListInput
		sub  string
	}{
		{"state", prListInput{RepositorySlug: "repo", State: "BOGUS"}, "state"},
		{"direction", prListInput{RepositorySlug: "repo", Direction: "BOGUS"}, "direction"},
		{"order", prListInput{RepositorySlug: "repo", Order: "BOGUS"}, "order"},
		{"role", prListInput{RepositorySlug: "repo", Participants: []prParticipantFilter{{Username: "u", Role: "BOGUS"}}}, "role"},
	} {
		assertValidation(t, svcNoReq.ListPullRequests(ctx, tc.in), tc.sub)
	}
}

// VP-015: P2-5/P2-6 activities fromId/fromType and PR commits withCounts.
func TestPRActivitiesAndCommitsParams(t *testing.T) {
	server, requests := newRecordingServer(t)
	svc := newTestService(server.URL, server.Client())
	ctx := context.Background()
	tru := true
	fromID := 42

	// Valid activities call: fromId + fromType=COMMENT.
	if out := svc.GetPullRequestActivities(ctx, prActivitiesInput{RepositorySlug: "repo", PullRequestID: 1, FromID: &fromID, FromType: "COMMENT"}); !out.Success {
		t.Fatalf("activities: %+v", out)
	}
	// PR commits with withCounts.
	if out := svc.GetPullRequestCommits(ctx, prCommitsInput{RepositorySlug: "repo", PullRequestID: 1, WithCounts: &tru}); !out.Success {
		t.Fatalf("commits: %+v", out)
	}

	if len(*requests) != 2 {
		t.Fatalf("requests = %v, want 2", *requests)
	}
	actReq := (*requests)[0]
	if !strings.Contains(actReq, "fromId=42") || !strings.Contains(actReq, "fromType=COMMENT") {
		t.Fatalf("activities request = %s, missing fromId/fromType", actReq)
	}
	if !strings.Contains((*requests)[1], "withCounts=true") {
		t.Fatalf("commits request = %s, missing withCounts=true", (*requests)[1])
	}

	// fromId without fromType → validation error.
	noReq := newNoRequestServer(t)
	svcNoReq := newTestService(noReq.URL, noReq.Client())
	assertValidation(t, svcNoReq.GetPullRequestActivities(ctx, prActivitiesInput{RepositorySlug: "repo", PullRequestID: 1, FromID: &fromID}), "fromType is required")

	// Bogus fromType → validation error.
	assertValidation(t, svcNoReq.GetPullRequestActivities(ctx, prActivitiesInput{RepositorySlug: "repo", PullRequestID: 1, FromType: "BOGUS"}), "fromType must be")
}

// VP-016: P2-7 PR changes changeScope validation and no start exposure.
func TestPRChangesChangeScope(t *testing.T) {
	server, requests := newRecordingServer(t)
	svc := newTestService(server.URL, server.Client())
	ctx := context.Background()

	// changeScope=ALL.
	if out := svc.GetPullRequestChanges(ctx, prChangesInput{RepositorySlug: "repo", PullRequestID: 1, ChangeScope: "ALL"}); !out.Success {
		t.Fatalf("ALL: %+v", out)
	}
	// RANGE with both IDs.
	if out := svc.GetPullRequestChanges(ctx, prChangesInput{RepositorySlug: "repo", PullRequestID: 1, ChangeScope: "RANGE", SinceID: "a", UntilID: "b"}); !out.Success {
		t.Fatalf("RANGE: %+v", out)
	}

	if len(*requests) != 2 {
		t.Fatalf("requests = %v, want 2", *requests)
	}
	if !strings.Contains((*requests)[0], "changeScope=ALL") {
		t.Fatalf("ALL request = %s, missing changeScope=ALL", (*requests)[0])
	}
	rangeReq := (*requests)[1]
	if !strings.Contains(rangeReq, "changeScope=RANGE") || !strings.Contains(rangeReq, "sinceId=a") || !strings.Contains(rangeReq, "untilId=b") {
		t.Fatalf("RANGE request = %s, missing params", rangeReq)
	}
	for _, req := range *requests {
		if strings.Contains(req, "start=") {
			t.Fatalf("request = %s, must not contain start", req)
		}
	}

	// RANGE missing untilId → validation error.
	noReq := newNoRequestServer(t)
	svcNoReq := newTestService(noReq.URL, noReq.Client())
	assertValidation(t, svcNoReq.GetPullRequestChanges(ctx, prChangesInput{RepositorySlug: "repo", PullRequestID: 1, ChangeScope: "RANGE", SinceID: "a"}), "sinceId and untilId")

	// Bogus changeScope → validation error.
	assertValidation(t, svcNoReq.GetPullRequestChanges(ctx, prChangesInput{RepositorySlug: "repo", PullRequestID: 1, ChangeScope: "BOGUS"}), "changeScope must be")
}

// VP-017: P2-8 PR diff diffType/withComments and hash-pair requirements.
func TestPRDiffDiffType(t *testing.T) {
	server, requests := newRecordingServer(t)
	svc := newTestService(server.URL, server.Client())
	ctx := context.Background()
	tru := true

	// EFFECTIVE.
	if out := svc.GetPullRequestDiff(ctx, prDiffInput{RepositorySlug: "repo", PullRequestID: 1, DiffType: "EFFECTIVE"}); !out.Success {
		t.Fatalf("EFFECTIVE: %+v", out)
	}
	// RANGE with sinceId+untilId.
	if out := svc.GetPullRequestDiff(ctx, prDiffInput{RepositorySlug: "repo", PullRequestID: 1, DiffType: "RANGE", SinceID: "a", UntilID: "b"}); !out.Success {
		t.Fatalf("RANGE: %+v", out)
	}
	// COMMIT with untilId + withComments.
	if out := svc.GetPullRequestDiff(ctx, prDiffInput{RepositorySlug: "repo", PullRequestID: 1, DiffType: "COMMIT", UntilID: "c", WithComments: &tru}); !out.Success {
		t.Fatalf("COMMIT: %+v", out)
	}

	if len(*requests) != 3 {
		t.Fatalf("requests = %v, want 3", *requests)
	}
	if !strings.Contains((*requests)[0], "diffType=EFFECTIVE") {
		t.Fatalf("EFFECTIVE request = %s, missing diffType", (*requests)[0])
	}
	rangeReq := (*requests)[1]
	if !strings.Contains(rangeReq, "diffType=RANGE") || !strings.Contains(rangeReq, "sinceId=a") || !strings.Contains(rangeReq, "untilId=b") {
		t.Fatalf("RANGE request = %s, missing params", rangeReq)
	}
	commitReq := (*requests)[2]
	if !strings.Contains(commitReq, "diffType=COMMIT") || !strings.Contains(commitReq, "untilId=c") || !strings.Contains(commitReq, "withComments=true") {
		t.Fatalf("COMMIT request = %s, missing params", commitReq)
	}

	// RANGE missing sinceId → validation error.
	noReq := newNoRequestServer(t)
	svcNoReq := newTestService(noReq.URL, noReq.Client())
	assertValidation(t, svcNoReq.GetPullRequestDiff(ctx, prDiffInput{RepositorySlug: "repo", PullRequestID: 1, DiffType: "RANGE", UntilID: "b"}), "sinceId and untilId")

	// COMMIT missing untilId → validation error.
	assertValidation(t, svcNoReq.GetPullRequestDiff(ctx, prDiffInput{RepositorySlug: "repo", PullRequestID: 1, DiffType: "COMMIT"}), "untilId is required")

	// Bogus diffType → validation error.
	assertValidation(t, svcNoReq.GetPullRequestDiff(ctx, prDiffInput{RepositorySlug: "repo", PullRequestID: 1, DiffType: "BOGUS"}), "diffType must be")
}

// VP-018: P2-9 comment anchor supports file and line modes with hash/diffType fields.
func TestPRCommentAnchorModes(t *testing.T) {
	var bodies []map[string]any
	var posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
	}))
	t.Cleanup(server.Close)
	svc := newTestService(server.URL, server.Client())
	ctx := context.Background()
	line := 10

	// File anchor (path only).
	if out := svc.AddPullRequestComment(ctx, commentInput{RepositorySlug: "repo", PullRequestID: 1, Text: "t", Anchor: &anchorInput{Path: "f.txt"}}); !out.Success {
		t.Fatalf("file anchor: %+v", out)
	}
	// Line anchor (path+line+lineType+fileType).
	if out := svc.AddPullRequestComment(ctx, commentInput{RepositorySlug: "repo", PullRequestID: 1, Text: "t", Anchor: &anchorInput{Path: "f.txt", Line: &line, LineType: "ADDED", FileType: "TO"}}); !out.Success {
		t.Fatalf("line anchor: %+v", out)
	}
	// Anchor with fromHash/toHash+diffType=RANGE.
	if out := svc.AddPullRequestComment(ctx, commentInput{RepositorySlug: "repo", PullRequestID: 1, Text: "t", Anchor: &anchorInput{Path: "f.txt", DiffType: "RANGE", FromHash: "aaa", ToHash: "bbb"}}); !out.Success {
		t.Fatalf("hash anchor: %+v", out)
	}

	if posts != 3 {
		t.Fatalf("posts = %d, want 3", posts)
	}
	// File anchor: no line/lineType/fileType in body.
	fileAnchor := bodies[0]["anchor"].(map[string]any)
	if fileAnchor["path"] != "f.txt" {
		t.Fatalf("file anchor path = %v", fileAnchor["path"])
	}
	if _, ok := fileAnchor["line"]; ok {
		t.Fatalf("file anchor must not contain line")
	}
	// Line anchor: line, lineType, fileType present.
	lineAnchor := bodies[1]["anchor"].(map[string]any)
	if lineAnchor["lineType"] != "ADDED" || lineAnchor["fileType"] != "TO" {
		t.Fatalf("line anchor = %v", lineAnchor)
	}
	// Hash anchor: diffType, fromHash, toHash present.
	hashAnchor := bodies[2]["anchor"].(map[string]any)
	if hashAnchor["diffType"] != "RANGE" || hashAnchor["fromHash"] != "aaa" || hashAnchor["toHash"] != "bbb" {
		t.Fatalf("hash anchor = %v", hashAnchor)
	}

	// Validation failures: no request.
	noReq := newNoRequestServer(t)
	svcNoReq := newTestService(noReq.URL, noReq.Client())
	for _, tc := range []struct {
		name   string
		anchor anchorInput
		sub    string
	}{
		{"missing path", anchorInput{}, "path"},
		{"line missing fileType", anchorInput{Path: "f.txt", Line: &line, LineType: "ADDED"}, "fileType"},
		{"file anchor with lineType", anchorInput{Path: "f.txt", LineType: "ADDED"}, "line"},
		{"fromHash without diffType", anchorInput{Path: "f.txt", FromHash: "aaa"}, "diffType"},
	} {
		assertValidation(t, svcNoReq.AddPullRequestComment(ctx, commentInput{RepositorySlug: "repo", PullRequestID: 1, Text: "t", Anchor: &tc.anchor}), tc.sub)
	}
}
