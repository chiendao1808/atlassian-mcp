package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/chiendao1808/atlassian-mcp/internal/result"
)

type getFileInput struct {
	RepositorySlug string `json:"repositorySlug"`
	Path           string `json:"path"`
	At             string `json:"at,omitempty"`
	Encoding       string `json:"encoding,omitempty"`
}

type commitListInput struct {
	RepositorySlug string `json:"repositorySlug"`
	Until          string `json:"until,omitempty"`
	Since          string `json:"since,omitempty"`
	Path           string `json:"path,omitempty"`
	Merges         string `json:"merges,omitempty"`
	FollowRenames  *bool  `json:"followRenames,omitempty"`
	WithCounts     *bool  `json:"withCounts,omitempty"`
	Start          *int   `json:"start,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
}

type commitInput struct {
	RepositorySlug string `json:"repositorySlug"`
	CommitID       string `json:"commitId"`
}

type commitPagedInput struct {
	RepositorySlug string `json:"repositorySlug"`
	CommitID       string `json:"commitId"`
	Start          *int   `json:"start,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
}

type diffInput struct {
	RepositorySlug string `json:"repositorySlug"`
	CommitID       string `json:"commitId,omitempty"`
	Path           string `json:"path,omitempty"`
	SrcPath        string `json:"srcPath,omitempty"`
	ContextLines   *int   `json:"contextLines,omitempty"`
	Whitespace     string `json:"whitespace,omitempty"`
}

type compareInput struct {
	RepositorySlug     string `json:"repositorySlug"`
	From               string `json:"from"`
	To                 string `json:"to"`
	FromRepositorySlug string `json:"fromRepositorySlug,omitempty"`
	Path               string `json:"path,omitempty"`
	SrcPath            string `json:"srcPath,omitempty"`
	ContextLines       *int   `json:"contextLines,omitempty"`
	Whitespace         string `json:"whitespace,omitempty"`
	Start              *int   `json:"start,omitempty"`
	Limit              *int   `json:"limit,omitempty"`
}

type commitFileInput struct {
	RepositorySlug string `json:"repositorySlug"`
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	Content        string `json:"content,omitempty"`
	ContentBase64  string `json:"contentBase64,omitempty"`
	Message        string `json:"message"`
	SourceCommitID string `json:"sourceCommitId,omitempty"`
	SourceBranch   string `json:"sourceBranch,omitempty"`
}

func (s *Service) GetFile(ctx context.Context, in getFileInput) result.Envelope {
	if in.Encoding != "" && in.Encoding != "text" && in.Encoding != "base64" {
		return fail("bitbucket_get_file", "encoding must be text or base64")
	}
	ep, err := s.client.RepositoryFileEndpoint(in.RepositorySlug, "/projects/{projectKey}/repos/{repositorySlug}/raw/{path}", "raw", in.Path)
	if err != nil {
		return fail("bitbucket_get_file", "repositorySlug and path are required")
	}
	raw, err := s.client.DoRaw(ctx, ep, q("at", in.At))
	if err != nil {
		return clientError("bitbucket_get_file", err)
	}
	encoding := in.Encoding
	if encoding == "" {
		encoding = "text"
	}
	// Auto mode returns text only for valid UTF-8; binary content stays lossless as base64.
	if encoding == "text" && utf8.Valid(raw.Body) {
		return result.OK("bitbucket", "bitbucket_get_file", map[string]any{"path": in.Path, "encoding": "text", "size": raw.Size, "content": string(raw.Body)})
	}
	return result.OK("bitbucket", "bitbucket_get_file", map[string]any{"path": in.Path, "encoding": "base64", "size": raw.Size, "content": base64.StdEncoding.EncodeToString(raw.Body)})
}

func (s *Service) ListCommits(ctx context.Context, in commitListInput) result.Envelope {
	if in.Merges != "" && in.Merges != "include" && in.Merges != "exclude" && in.Merges != "only" {
		return fail("bitbucket_list_commits", "merges must be include, exclude, or only")
	}
	return s.getJSON(ctx, "bitbucket_list_commits", in.RepositorySlug, "commits", q(
		"until", in.Until, "since", in.Since, "path", in.Path, "merges", in.Merges,
	).bool("followRenames", in.FollowRenames).bool("withCounts", in.WithCounts).page(in.Start, in.Limit), "commits")
}

func (s *Service) GetCommit(ctx context.Context, in commitInput) result.Envelope {
	if strings.TrimSpace(in.CommitID) == "" {
		return fail("bitbucket_get_commit", "commitId is required")
	}
	return s.getJSONSegments(ctx, "bitbucket_get_commit", in.RepositorySlug, []string{"commits", in.CommitID}, nil, "commit")
}

func (s *Service) GetCommitChanges(ctx context.Context, in commitPagedInput) result.Envelope {
	if strings.TrimSpace(in.CommitID) == "" {
		return fail("bitbucket_get_commit_changes", "commitId is required")
	}
	return s.getJSONSegments(ctx, "bitbucket_get_commit_changes", in.RepositorySlug, []string{"commits", in.CommitID, "changes"}, q().page(in.Start, in.Limit), "changes")
}

func (s *Service) GetCommitDiff(ctx context.Context, in diffInput) result.Envelope {
	if strings.TrimSpace(in.CommitID) == "" {
		return fail("bitbucket_get_commit_diff", "commitId is required")
	}
	return s.diffSegments(ctx, "bitbucket_get_commit_diff", in.RepositorySlug, []string{"commits", in.CommitID, "diff"}, in.Path, q("srcPath", in.SrcPath, "whitespace", in.Whitespace).int("contextLines", in.ContextLines))
}

func (s *Service) CompareCommits(ctx context.Context, in compareInput) result.Envelope {
	if strings.TrimSpace(in.From) == "" || strings.TrimSpace(in.To) == "" {
		return fail("bitbucket_compare_commits", "from and to are required")
	}
	return s.getJSON(ctx, "bitbucket_compare_commits", in.RepositorySlug, "compare/commits", compareq(in).page(in.Start, in.Limit), "commits")
}

func (s *Service) CompareChanges(ctx context.Context, in compareInput) result.Envelope {
	if strings.TrimSpace(in.From) == "" || strings.TrimSpace(in.To) == "" {
		return fail("bitbucket_compare_changes", "from and to are required")
	}
	return s.getJSON(ctx, "bitbucket_compare_changes", in.RepositorySlug, "compare/changes", compareq(in).page(in.Start, in.Limit), "changes")
}

func (s *Service) CompareDiff(ctx context.Context, in compareInput) result.Envelope {
	if strings.TrimSpace(in.From) == "" || strings.TrimSpace(in.To) == "" {
		return fail("bitbucket_compare_diff", "from and to are required")
	}
	return s.diff(ctx, "bitbucket_compare_diff", in.RepositorySlug, "compare/diff", in.Path, compareq(in).add("srcPath", in.SrcPath).add("whitespace", in.Whitespace).int("contextLines", in.ContextLines))
}

func (s *Service) CommitFile(ctx context.Context, in commitFileInput) result.Envelope {
	if strings.TrimSpace(in.Path) == "" || strings.TrimSpace(in.Branch) == "" || strings.TrimSpace(in.Message) == "" {
		return fail("bitbucket_commit_file", "path, branch, and message are required")
	}
	// Exactly one content source avoids silently committing a different payload than requested.
	if (in.Content == "") == (in.ContentBase64 == "") {
		return fail("bitbucket_commit_file", "exactly one of content or contentBase64 is required")
	}
	content := []byte(in.Content)
	if in.ContentBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(in.ContentBase64)
		if err != nil {
			return fail("bitbucket_commit_file", "contentBase64 is not valid base64")
		}
		content = decoded
	}
	fields := map[string]string{"branch": in.Branch, "message": in.Message}
	if in.SourceCommitID != "" {
		fields["sourceCommitId"] = in.SourceCommitID
	}
	if in.SourceBranch != "" {
		fields["sourceBranch"] = in.SourceBranch
	}
	ep, err := s.client.RepositoryFileEndpoint(in.RepositorySlug, "/projects/{projectKey}/repos/{repositorySlug}/browse/{path}", "browse", in.Path)
	if err != nil {
		return fail("bitbucket_commit_file", "repositorySlug and path are required")
	}
	var out map[string]any
	if err := s.client.DoMultipart(ctx, http.MethodPut, ep, fields, "content", in.Path, bytes.NewReader(content), &out); err != nil {
		return clientError("bitbucket_commit_file", err)
	}
	out["singleFileCommit"] = true
	return result.OK("bitbucket", "bitbucket_commit_file", out)
}

func compareq(in compareInput) query {
	return q("from", in.From, "to", in.To, "fromRepositorySlug", in.FromRepositorySlug)
}
