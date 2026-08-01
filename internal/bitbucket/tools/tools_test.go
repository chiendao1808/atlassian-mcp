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

func intPtr(v int) *int { return &v }
