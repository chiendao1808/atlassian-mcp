package tools

import (
	"context"
	"strconv"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/result"
)

type prListInput struct {
	RepositorySlug string                `json:"repositorySlug"`
	State          string                `json:"state,omitempty"`
	Direction      string                `json:"direction,omitempty"`
	At             string                `json:"at,omitempty"`
	Order          string                `json:"order,omitempty"`
	Participants   []prParticipantFilter `json:"participants,omitempty"`
	WithAttributes *bool                 `json:"withAttributes,omitempty"`
	WithProperties *bool                 `json:"withProperties,omitempty"`
	Start          *int                  `json:"start,omitempty"`
	Limit          *int                  `json:"limit,omitempty"`
}

// prParticipantFilter is one entry of the PR list participant filter set;
// filters serialize consecutively as username.N / role.N / approved.N with N
// starting at 1, no gaps, and a maximum of 10 (guide §3.14).
type prParticipantFilter struct {
	Username string `json:"username"`
	Role     string `json:"role,omitempty"` // AUTHOR|REVIEWER|PARTICIPANT
	Approved *bool  `json:"approved,omitempty"`
}

type prInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
}

type prActivitiesInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
	FromID         *int   `json:"fromId,omitempty"`
	FromType       string `json:"fromType,omitempty"` // COMMENT|ACTIVITY; required when fromId is present
	Start          *int   `json:"start,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
}

type prCommitsInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
	WithCounts     *bool  `json:"withCounts,omitempty"`
	Start          *int   `json:"start,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
}

type prChangesInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
	ChangeScope    string `json:"changeScope,omitempty"` // ALL|UNREVIEWED|RANGE
	SinceID        string `json:"sinceId,omitempty"`
	UntilID        string `json:"untilId,omitempty"`
	WithComments   *bool  `json:"withComments,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
}

type prDiffInput struct {
	RepositorySlug string `json:"repositorySlug"`
	PullRequestID  int    `json:"pullRequestId"`
	Path           string `json:"path,omitempty"`
	SrcPath        string `json:"srcPath,omitempty"`
	DiffType       string `json:"diffType,omitempty"` // EFFECTIVE|RANGE|COMMIT
	SinceID        string `json:"sinceId,omitempty"`
	UntilID        string `json:"untilId,omitempty"`
	ContextLines   *int   `json:"contextLines,omitempty"`
	Whitespace     string `json:"whitespace,omitempty"`
	WithComments   *bool  `json:"withComments,omitempty"`
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
	DiffType string `json:"diffType,omitempty"`
	FromHash string `json:"fromHash,omitempty"`
	ToHash   string `json:"toHash,omitempty"`
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

// updatePRInput updates an existing pull request's editable metadata using
// auto-preserve semantics. Any pointer field left nil is preserved from the
// current PR (fetched via one GET immediately before the PUT); a non-nil field
// overrides. Reviewers is a pointer-to-slice so a nil pointer ("leave reviewers
// untouched") is distinguishable from a non-nil empty slice ("clear all
// reviewers"). The optimistic-locking version is never supplied by the caller —
// it is always read fresh from the pre-PUT GET.
type updatePRInput struct {
	RepositorySlug string           `json:"repositorySlug"`
	PullRequestID  int              `json:"pullRequestId"`
	Title          *string          `json:"title,omitempty"`
	Description    *string          `json:"description,omitempty"`
	Reviewers      *[]reviewerInput `json:"reviewers,omitempty"`
}

func (s *Service) ListPullRequests(ctx context.Context, in prListInput) result.Envelope {
	const tool = "bitbucket_list_pull_requests"
	if in.State != "" && in.State != "OPEN" && in.State != "DECLINED" && in.State != "MERGED" && in.State != "ALL" {
		return fail(tool, "state must be OPEN, DECLINED, MERGED, or ALL")
	}
	if in.Direction != "" && in.Direction != "INCOMING" && in.Direction != "OUTGOING" {
		return fail(tool, "direction must be INCOMING or OUTGOING")
	}
	if in.Order != "" && in.Order != "OLDEST" && in.Order != "NEWEST" {
		return fail(tool, "order must be OLDEST or NEWEST")
	}
	if len(in.Participants) > 10 {
		return fail(tool, "at most 10 participant filters are supported")
	}
	qq := q("state", in.State, "direction", in.Direction, "at", in.At, "order", in.Order)
	// Participant filters serialize consecutively as username.N / role.N /
	// approved.N with N starting at 1 and no gaps (guide §3.14); a filter
	// without a username would create a gap, so it is rejected up front.
	for i, p := range in.Participants {
		if strings.TrimSpace(p.Username) == "" {
			return fail(tool, "participants[].username is required")
		}
		if p.Role != "" && p.Role != "AUTHOR" && p.Role != "REVIEWER" && p.Role != "PARTICIPANT" {
			return fail(tool, "participants[].role must be AUTHOR, REVIEWER, or PARTICIPANT")
		}
		n := strconv.Itoa(i + 1)
		qq = qq.add("username."+n, p.Username).add("role."+n, p.Role).bool("approved."+n, p.Approved)
	}
	return s.getJSON(ctx, tool, in.RepositorySlug, "pull-requests", qq.bool("withAttributes", in.WithAttributes).bool("withProperties", in.WithProperties).page(in.Start, in.Limit), "pullRequests")
}

func (s *Service) GetPullRequest(ctx context.Context, in prInput) result.Envelope {
	return s.getPR(ctx, "bitbucket_get_pull_request", in.RepositorySlug, in.PullRequestID, "pullRequest")
}

func (s *Service) GetPullRequestActivities(ctx context.Context, in prActivitiesInput) result.Envelope {
	const tool = "bitbucket_get_pull_request_activities"
	if in.FromType != "" && in.FromType != "COMMENT" && in.FromType != "ACTIVITY" {
		return fail(tool, "fromType must be COMMENT or ACTIVITY")
	}
	if in.FromID != nil && strings.TrimSpace(in.FromType) == "" {
		return fail(tool, "fromType is required when fromId is present")
	}
	return s.getJSON(ctx, tool, in.RepositorySlug, prPath(in.PullRequestID, "activities"), q().int("fromId", in.FromID).add("fromType", in.FromType).page(in.Start, in.Limit), "activities")
}

func (s *Service) GetPullRequestCommits(ctx context.Context, in prCommitsInput) result.Envelope {
	return s.getJSON(ctx, "bitbucket_get_pull_request_commits", in.RepositorySlug, prPath(in.PullRequestID, "commits"), q().bool("withCounts", in.WithCounts).page(in.Start, in.Limit), "commits")
}

func (s *Service) GetPullRequestChanges(ctx context.Context, in prChangesInput) result.Envelope {
	const tool = "bitbucket_get_pull_request_changes"
	if in.ChangeScope != "" && in.ChangeScope != "ALL" && in.ChangeScope != "UNREVIEWED" && in.ChangeScope != "RANGE" {
		return fail(tool, "changeScope must be ALL, UNREVIEWED, or RANGE")
	}
	if in.ChangeScope == "RANGE" && (strings.TrimSpace(in.SinceID) == "" || strings.TrimSpace(in.UntilID) == "") {
		return fail(tool, "sinceId and untilId are required when changeScope is RANGE")
	}
	// start is intentionally not exposed: Bitbucket 5.10.2 ignores it on this
	// endpoint (guide §3.18).
	return s.getJSON(ctx, tool, in.RepositorySlug, prPath(in.PullRequestID, "changes"), q("changeScope", in.ChangeScope, "sinceId", in.SinceID, "untilId", in.UntilID).bool("withComments", in.WithComments).int("limit", in.Limit), "changes")
}

func (s *Service) GetPullRequestDiff(ctx context.Context, in prDiffInput) result.Envelope {
	const tool = "bitbucket_get_pull_request_diff"
	if in.DiffType != "" && in.DiffType != "EFFECTIVE" && in.DiffType != "RANGE" && in.DiffType != "COMMIT" {
		return fail(tool, "diffType must be EFFECTIVE, RANGE, or COMMIT")
	}
	if in.DiffType == "RANGE" && (strings.TrimSpace(in.SinceID) == "" || strings.TrimSpace(in.UntilID) == "") {
		return fail(tool, "sinceId and untilId are required when diffType is RANGE")
	}
	if in.DiffType == "COMMIT" && strings.TrimSpace(in.UntilID) == "" {
		return fail(tool, "untilId is required when diffType is COMMIT")
	}
	return s.diff(ctx, tool, in.RepositorySlug, prPath(in.PullRequestID, "diff"), in.Path, q("srcPath", in.SrcPath, "diffType", in.DiffType, "sinceId", in.SinceID, "untilId", in.UntilID, "whitespace", in.Whitespace).int("contextLines", in.ContextLines).bool("withComments", in.WithComments))
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
		if strings.TrimSpace(in.Anchor.Path) == "" {
			return fail("bitbucket_add_pull_request_comment", "anchor requires path")
		}
		if in.Anchor.Line != nil {
			// Line anchors need both qualifiers fully populated.
			if in.Anchor.LineType != "ADDED" && in.Anchor.LineType != "REMOVED" && in.Anchor.LineType != "CONTEXT" {
				return fail("bitbucket_add_pull_request_comment", "line anchor requires lineType ADDED, REMOVED, or CONTEXT")
			}
			if in.Anchor.FileType != "FROM" && in.Anchor.FileType != "TO" {
				return fail("bitbucket_add_pull_request_comment", "line anchor requires fileType FROM or TO")
			}
		} else if in.Anchor.LineType != "" || in.Anchor.FileType != "" {
			// Partially populated anchors are never sent (guide §3.22):
			// lineType/fileType only make sense together with line.
			return fail("bitbucket_add_pull_request_comment", "lineType and fileType require line")
		}
		if (in.Anchor.FromHash != "" || in.Anchor.ToHash != "") && in.Anchor.DiffType == "" {
			return fail("bitbucket_add_pull_request_comment", "diffType is required when fromHash or toHash is supplied")
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
	body := map[string]any{
		"user":     map[string]any{"name": s.userSlug},
		"status":   status,
		"approved": status == "APPROVED",
	}
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

// UpdatePullRequest edits an existing PR's title, description, and reviewers.
// It auto-fetches the current PR first (mirroring transition()'s auto-fetch
// idiom) so the optimistic-locking version and any omitted fields are sourced
// fresh rather than trusted from the caller, then issues a single JSON-body
// PUT. On a 409 (stale version / concurrent edit) it surfaces the conflict via
// the shared clientError/FailHTTPDetail path and never retries.
func (s *Service) UpdatePullRequest(ctx context.Context, in updatePRInput) result.Envelope {
	tool := "bitbucket_update_pull_request"
	// Auto-fetch the current PR: getPR guards pullRequestId<=0 and (via
	// endpoint) repositorySlug, and gives us the fresh version plus the current
	// title/description/reviewers to preserve. Propagate a failed GET verbatim,
	// exactly like transition() does (pull_requests.go:203-206).
	env := s.getPR(ctx, tool, in.RepositorySlug, in.PullRequestID, "pullRequest")
	if !env.Success {
		return env
	}
	pr := env.Data.(map[string]any)["pullRequest"].(map[string]any)

	body := map[string]any{}
	// version is always sourced fresh from the GET (never caller-supplied).
	raw, ok := pr["version"].(float64)
	if !ok {
		return fail(tool, "current PR version could not be read")
	}
	body["version"] = int(raw)
	// title: caller override, else preserve current.
	if in.Title != nil {
		body["title"] = *in.Title
	} else if v, ok := pr["title"]; ok {
		body["title"] = v
	}
	// description: caller override, else preserve current when present.
	if in.Description != nil {
		body["description"] = *in.Description
	} else if v, ok := pr["description"]; ok {
		body["description"] = v
	}
	// reviewers: caller override (non-nil, incl. empty slice = clear all), else
	// preserve the current reviewer set, normalized to the write shape
	// ({"user":{"name":...}}) rather than echoed as the full GET participant
	// object (which also carries role/approved/lastReviewedCommit and is not
	// guaranteed accepted on write).
	if in.Reviewers != nil {
		body["reviewers"] = *in.Reviewers
	} else {
		body["reviewers"] = normalizeReviewers(pr["reviewers"])
	}

	return s.putJSON(ctx, tool, in.RepositorySlug, prPath(in.PullRequestID), nil, body, "pullRequest")
}

// normalizeReviewers reduces GET's full participant objects
// ({"user":{"name":...},"role":"REVIEWER","approved":...,...}) to the minimal
// write shape Bitbucket's PUT expects ({"user":{"name":...}}), dropping
// read-only fields the write endpoint does not accept. It always returns a
// non-nil (possibly empty) slice so callers can serialize an explicit
// "reviewers": [] rather than a missing key.
//
// This function is only invoked on the "caller omitted reviewers" path in
// UpdatePullRequest, where the intent is to leave the current reviewer set
// untouched. If a participant's user.name can't be read as a usable
// non-empty string (missing, wrong type, or empty), the entry is NOT dropped:
// its original "user" sub-object is preserved verbatim instead, whatever
// shape it has. Dropping it would silently remove that reviewer as a side
// effect of an update that never intended to touch reviewers at all. An
// entry is skipped entirely only when it isn't a participant object in the
// first place — i.e. not a map[string]any, or with no "user" key at all —
// because there is nothing to preserve in that case.
func normalizeReviewers(raw any) []map[string]any {
	list, _ := raw.([]any)
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		participant, ok := item.(map[string]any)
		if !ok {
			continue
		}
		userRaw, hasUser := participant["user"]
		if !hasUser {
			continue
		}
		if user, ok := userRaw.(map[string]any); ok {
			if name, ok := user["name"].(string); ok && name != "" {
				out = append(out, map[string]any{"user": map[string]any{"name": name}})
				continue
			}
		}
		// Fallback-preserve: keep the reviewer with its original user
		// sub-object rather than dropping it, since name normalization
		// failed but this path must never silently remove reviewers.
		out = append(out, map[string]any{"user": userRaw})
	}
	return out
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
