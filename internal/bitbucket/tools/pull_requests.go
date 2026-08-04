package tools

import (
	"context"
	"strconv"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/result"
)

type prListInput struct {
	RepositorySlug string `json:"repositorySlug"`
	State          string `json:"state,omitempty"`
	Direction      string `json:"direction,omitempty"`
	At             string `json:"at,omitempty"`
	Order          string `json:"order,omitempty"`
	Participant    string `json:"participant,omitempty"`
	Start          *int   `json:"start,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
}

type prInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
}

type prPagedInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
	Start          *int   `json:"start,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
}

type prChangesInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
	SinceID        string `json:"sinceId,omitempty"`
	UntilID        string `json:"untilId,omitempty"`
	WithComments   *bool  `json:"withComments,omitempty"`
	Start          *int   `json:"start,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
}

type prDiffInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
	Path           string `json:"path,omitempty"`
	SrcPath        string `json:"srcPath,omitempty"`
	SinceID        string `json:"sinceId,omitempty"`
	UntilID        string `json:"untilId,omitempty"`
	ContextLines   *int   `json:"contextLines,omitempty"`
	Whitespace     string `json:"whitespace,omitempty"`
}

type reviewerInput struct {
	User struct {
		Name string `json:"name,omitempty"`
		Slug string `json:"slug,omitempty"`
	} `json:"user"`
}

type createPRInput struct {
	RepositorySlug     string          `json:"repositorySlug"`
	Title              string          `json:"title"`
	Description        string          `json:"description,omitempty"`
	FromBranch         string          `json:"fromBranch"`
	ToBranch           string          `json:"toBranch"`
	FromRepositorySlug string          `json:"fromRepositorySlug,omitempty"`
	Reviewers          []reviewerInput `json:"reviewers,omitempty"`
}

type anchorInput struct {
	Path     string `json:"path,omitempty"`
	SrcPath  string `json:"srcPath,omitempty"`
	Line     *int   `json:"line,omitempty"`
	LineType string `json:"lineType,omitempty"`
	FileType string `json:"fileType,omitempty"`
}

type commentInput struct {
	RepositorySlug string       `json:"repositorySlug"`
	PullRequestID  int          `json:"pullRequestId"`
	Text           string       `json:"text"`
	ParentID       *int         `json:"parentId,omitempty"`
	Anchor         *anchorInput `json:"anchor,omitempty"`
}

type reviewStatusInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
	Status         string `json:"status"`
}

type transitionInput struct {
	RepositorySlug  string `json:"repositorySlug"`
	PullRequestID   int    `json:"pullRequestId"`
	ExpectedVersion *int   `json:"expectedVersion,omitempty"`
	Precheck        *bool  `json:"precheck,omitempty"`
}

func (s *Service) ListPullRequests(ctx context.Context, in prListInput) result.Envelope {
	return s.getJSON(ctx, "bitbucket_list_pull_requests", in.RepositorySlug, "pull-requests", q(
		"state", in.State, "direction", in.Direction, "at", in.At, "order", in.Order, "participant", in.Participant,
	).page(in.Start, in.Limit), "pullRequests")
}

func (s *Service) GetPullRequest(ctx context.Context, in prInput) result.Envelope {
	return s.getPR(ctx, "bitbucket_get_pull_request", in.RepositorySlug, in.PullRequestID, "pullRequest")
}

func (s *Service) GetPullRequestActivities(ctx context.Context, in prPagedInput) result.Envelope {
	return s.getJSON(ctx, "bitbucket_get_pull_request_activities", in.RepositorySlug, prPath(in.PullRequestID, "activities"), q().page(in.Start, in.Limit), "activities")
}

func (s *Service) GetPullRequestCommits(ctx context.Context, in prPagedInput) result.Envelope {
	return s.getJSON(ctx, "bitbucket_get_pull_request_commits", in.RepositorySlug, prPath(in.PullRequestID, "commits"), q().page(in.Start, in.Limit), "commits")
}

func (s *Service) GetPullRequestChanges(ctx context.Context, in prChangesInput) result.Envelope {
	return s.getJSON(ctx, "bitbucket_get_pull_request_changes", in.RepositorySlug, prPath(in.PullRequestID, "changes"), q("sinceId", in.SinceID, "untilId", in.UntilID).bool("withComments", in.WithComments).page(in.Start, in.Limit), "changes")
}

func (s *Service) GetPullRequestDiff(ctx context.Context, in prDiffInput) result.Envelope {
	return s.diff(ctx, "bitbucket_get_pull_request_diff", in.RepositorySlug, prPath(in.PullRequestID, "diff"), in.Path, q("srcPath", in.SrcPath, "sinceId", in.SinceID, "untilId", in.UntilID, "whitespace", in.Whitespace).int("contextLines", in.ContextLines))
}

func (s *Service) CheckPullRequestMergeability(ctx context.Context, in prInput) result.Envelope {
	return s.getJSON(ctx, "bitbucket_check_pull_request_mergeability", in.RepositorySlug, prPath(in.PullRequestID, "merge"), nil, "mergeability")
}

func (s *Service) CreatePullRequest(ctx context.Context, in createPRInput) result.Envelope {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.FromBranch) == "" || strings.TrimSpace(in.ToBranch) == "" {
		return fail("bitbucket_create_pull_request", "title, fromBranch, and toBranch are required")
	}
	fromRepo := in.RepositorySlug
	if in.FromRepositorySlug != "" {
		fromRepo = in.FromRepositorySlug
	}
	projectKey := s.client.ProjectKey()
	body := map[string]any{
		"title":       in.Title,
		"description": in.Description,
		"fromRef":     ref(in.FromBranch, fromRepo, projectKey),
		"toRef":       ref(in.ToBranch, in.RepositorySlug, projectKey),
	}
	if len(in.Reviewers) > 0 {
		body["reviewers"] = in.Reviewers
	}
	return s.postJSON(ctx, "bitbucket_create_pull_request", in.RepositorySlug, "pull-requests", nil, body, "pullRequest")
}

func (s *Service) AddPullRequestComment(ctx context.Context, in commentInput) result.Envelope {
	if strings.TrimSpace(in.Text) == "" {
		return fail("bitbucket_add_pull_request_comment", "text is required")
	}
	body := map[string]any{"text": in.Text}
	if in.ParentID != nil {
		body["parent"] = map[string]any{"id": *in.ParentID}
	}
	if in.Anchor != nil {
		if in.Anchor.Path == "" || in.Anchor.Line == nil || in.Anchor.LineType == "" || in.Anchor.FileType == "" {
			return fail("bitbucket_add_pull_request_comment", "inline anchor requires path, line, lineType, and fileType")
		}
		body["anchor"] = in.Anchor
	}
	return s.postJSON(ctx, "bitbucket_add_pull_request_comment", in.RepositorySlug, prPath(in.PullRequestID, "comments"), nil, body, "comment")
}

func (s *Service) SetPullRequestReviewStatus(ctx context.Context, in reviewStatusInput) result.Envelope {
	if strings.TrimSpace(s.userSlug) == "" {
		return result.Fail("bitbucket", "bitbucket_set_pull_request_review_status", "BITBUCKET_REVIEW_IDENTITY_REQUIRED", "BITBUCKET_USER_SLUG is required for review status updates")
	}
	status := strings.ToUpper(strings.TrimSpace(in.Status))
	if status != "APPROVED" && status != "NEEDS_WORK" && status != "UNAPPROVED" {
		return fail("bitbucket_set_pull_request_review_status", "status must be APPROVED, NEEDS_WORK, or UNAPPROVED")
	}
	body := map[string]any{"status": status, "approved": status == "APPROVED"}
	return s.putJSON(ctx, "bitbucket_set_pull_request_review_status", in.RepositorySlug, prPath(in.PullRequestID, "participants", s.userSlug), nil, body, "participant")
}

func (s *Service) MergePullRequest(ctx context.Context, in transitionInput) result.Envelope {
	if in.Precheck == nil || *in.Precheck {
		check := s.CheckPullRequestMergeability(ctx, prInput{RepositorySlug: in.RepositorySlug, PullRequestID: in.PullRequestID})
		if !check.Success {
			return check
		}
	}
	return s.transition(ctx, "bitbucket_merge_pull_request", in, "merge")
}

func (s *Service) DeclinePullRequest(ctx context.Context, in transitionInput) result.Envelope {
	return s.transition(ctx, "bitbucket_decline_pull_request", in, "decline")
}

func (s *Service) ReopenPullRequest(ctx context.Context, in transitionInput) result.Envelope {
	return s.transition(ctx, "bitbucket_reopen_pull_request", in, "reopen")
}

func (s *Service) transition(ctx context.Context, tool string, in transitionInput, action string) result.Envelope {
	version := in.ExpectedVersion
	if version == nil {
		// Bitbucket PR transitions require the current version; read it once when callers omit it.
		env := s.getPR(ctx, tool, in.RepositorySlug, in.PullRequestID, "pullRequest")
		if !env.Success {
			return env
		}
		pr := env.Data.(map[string]any)["pullRequest"].(map[string]any)
		if raw, ok := pr["version"].(float64); ok {
			v := int(raw)
			version = &v
		}
	}
	if version == nil {
		return fail(tool, "expectedVersion is required when the current PR version cannot be read")
	}
	return s.postJSON(ctx, tool, in.RepositorySlug, prPath(in.PullRequestID, action), q("version", strconv.Itoa(*version)), nil, "pullRequest")
}

func (s *Service) getPR(ctx context.Context, tool, slug string, id int, key string) result.Envelope {
	if id <= 0 {
		return fail(tool, "pullRequestId is required")
	}
	return s.getJSON(ctx, tool, slug, prPath(id), nil, key)
}

func prPath(id int, parts ...string) string {
	all := append([]string{"pull-requests", strconv.Itoa(id)}, parts...)
	return strings.Join(all, "/")
}

func ref(branch, repo, projectKey string) map[string]any {
	id := branch
	if !strings.HasPrefix(id, "refs/heads/") {
		// Normalize short branch names while preserving already-qualified Bitbucket ref IDs.
		id = "refs/heads/" + strings.TrimPrefix(id, "/")
	}
	return map[string]any{
		"id": id,
		"repository": map[string]any{
			"slug":    repo,
			"project": map[string]any{"key": projectKey},
		},
	}
}
