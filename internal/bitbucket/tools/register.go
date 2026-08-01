package tools

import (
	"context"

	"github.com/chiendao1808/atlassian-mcp/internal/result"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Definitions() []*mcp.Tool {
	open := true
	additive := false
	destructive := true
	names := []struct {
		name        string
		description string
		read        bool
		destructive *bool
	}{
		// Branch and repository tools.
		{"bitbucket_get_repository", "Read Bitbucket repository metadata.", true, nil},
		{"bitbucket_list_branches", "List Bitbucket branches with optional filters and paging.", true, nil},
		{"bitbucket_get_default_branch", "Read the default Bitbucket branch.", true, nil},
		{"bitbucket_create_branch", "Create one Bitbucket branch.", false, &additive},

		// Commit, file, and compare tools.
		{"bitbucket_get_file", "Read one Bitbucket file as text or base64.", true, nil},
		{"bitbucket_list_commits", "List Bitbucket commits with optional filters and paging.", true, nil},
		{"bitbucket_get_commit", "Read one Bitbucket commit.", true, nil},
		{"bitbucket_get_commit_changes", "List changed paths for one Bitbucket commit.", true, nil},
		{"bitbucket_get_commit_diff", "Read structured diff for one Bitbucket commit.", true, nil},
		{"bitbucket_compare_commits", "Compare commits between refs.", true, nil},
		{"bitbucket_compare_changes", "Compare changed paths between refs.", true, nil},
		{"bitbucket_compare_diff", "Compare structured diff between refs.", true, nil},
		{"bitbucket_commit_file", "Create or update one file with one Bitbucket commit.", false, &destructive},

		// Pull request tools.
		{"bitbucket_list_pull_requests", "List Bitbucket pull requests.", true, nil},
		{"bitbucket_get_pull_request", "Read one Bitbucket pull request.", true, nil},
		{"bitbucket_get_pull_request_activities", "List Bitbucket pull request activities.", true, nil},
		{"bitbucket_get_pull_request_commits", "List Bitbucket pull request commits.", true, nil},
		{"bitbucket_get_pull_request_changes", "List Bitbucket pull request changed paths.", true, nil},
		{"bitbucket_get_pull_request_diff", "Read Bitbucket pull request diff.", true, nil},
		{"bitbucket_check_pull_request_mergeability", "Check Bitbucket pull request mergeability.", true, nil},
		{"bitbucket_create_pull_request", "Create one Bitbucket pull request.", false, &additive},
		{"bitbucket_add_pull_request_comment", "Add one Bitbucket pull request comment.", false, &additive},
		{"bitbucket_set_pull_request_review_status", "Set the configured Bitbucket user's PR review status.", false, &additive},
		{"bitbucket_merge_pull_request", "Merge one Bitbucket pull request with version safety.", false, &destructive},
		{"bitbucket_decline_pull_request", "Decline one Bitbucket pull request with version safety.", false, &destructive},
		{"bitbucket_reopen_pull_request", "Reopen one Bitbucket pull request with version safety.", false, &destructive},
	}
	defs := make([]*mcp.Tool, 0, len(names))
	for _, item := range names {
		def := &mcp.Tool{Name: item.name, Description: item.description, Annotations: &mcp.ToolAnnotations{OpenWorldHint: &open}}
		if item.read {
			def.Annotations.ReadOnlyHint = true
		}
		if item.destructive != nil {
			def.Annotations.DestructiveHint = item.destructive
		}
		defs = append(defs, def)
	}
	return defs
}

func (s *Service) Register(server *mcp.Server) {
	defs := Definitions()
	s.registerBranchTools(server, defs)
	s.registerCommitTools(server, defs)
	s.registerPullRequestTools(server, defs)
}

func (s *Service) registerBranchTools(server *mcp.Server, defs []*mcp.Tool) {
	mcp.AddTool(server, defs[0], func(ctx context.Context, req *mcp.CallToolRequest, input repositoryInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetRepository(ctx, input), nil
	})
	mcp.AddTool(server, defs[1], func(ctx context.Context, req *mcp.CallToolRequest, input listBranchesInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.ListBranches(ctx, input), nil
	})
	mcp.AddTool(server, defs[2], func(ctx context.Context, req *mcp.CallToolRequest, input repositoryInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetDefaultBranch(ctx, input), nil
	})
	mcp.AddTool(server, defs[3], func(ctx context.Context, req *mcp.CallToolRequest, input createBranchInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CreateBranch(ctx, input), nil
	})
}

func (s *Service) registerCommitTools(server *mcp.Server, defs []*mcp.Tool) {
	mcp.AddTool(server, defs[4], func(ctx context.Context, req *mcp.CallToolRequest, input getFileInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetFile(ctx, input), nil
	})
	mcp.AddTool(server, defs[5], func(ctx context.Context, req *mcp.CallToolRequest, input commitListInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.ListCommits(ctx, input), nil
	})
	mcp.AddTool(server, defs[6], func(ctx context.Context, req *mcp.CallToolRequest, input commitInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetCommit(ctx, input), nil
	})
	mcp.AddTool(server, defs[7], func(ctx context.Context, req *mcp.CallToolRequest, input commitPagedInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetCommitChanges(ctx, input), nil
	})
	mcp.AddTool(server, defs[8], func(ctx context.Context, req *mcp.CallToolRequest, input diffInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetCommitDiff(ctx, input), nil
	})
	mcp.AddTool(server, defs[9], func(ctx context.Context, req *mcp.CallToolRequest, input compareInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CompareCommits(ctx, input), nil
	})
	mcp.AddTool(server, defs[10], func(ctx context.Context, req *mcp.CallToolRequest, input compareInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CompareChanges(ctx, input), nil
	})
	mcp.AddTool(server, defs[11], func(ctx context.Context, req *mcp.CallToolRequest, input compareInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CompareDiff(ctx, input), nil
	})
	mcp.AddTool(server, defs[12], func(ctx context.Context, req *mcp.CallToolRequest, input commitFileInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CommitFile(ctx, input), nil
	})
}

func (s *Service) registerPullRequestTools(server *mcp.Server, defs []*mcp.Tool) {
	mcp.AddTool(server, defs[13], func(ctx context.Context, req *mcp.CallToolRequest, input prListInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.ListPullRequests(ctx, input), nil
	})
	mcp.AddTool(server, defs[14], func(ctx context.Context, req *mcp.CallToolRequest, input prInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetPullRequest(ctx, input), nil
	})
	mcp.AddTool(server, defs[15], func(ctx context.Context, req *mcp.CallToolRequest, input prPagedInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetPullRequestActivities(ctx, input), nil
	})
	mcp.AddTool(server, defs[16], func(ctx context.Context, req *mcp.CallToolRequest, input prPagedInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetPullRequestCommits(ctx, input), nil
	})
	mcp.AddTool(server, defs[17], func(ctx context.Context, req *mcp.CallToolRequest, input prChangesInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetPullRequestChanges(ctx, input), nil
	})
	mcp.AddTool(server, defs[18], func(ctx context.Context, req *mcp.CallToolRequest, input prDiffInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetPullRequestDiff(ctx, input), nil
	})
	mcp.AddTool(server, defs[19], func(ctx context.Context, req *mcp.CallToolRequest, input prInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CheckPullRequestMergeability(ctx, input), nil
	})
	mcp.AddTool(server, defs[20], func(ctx context.Context, req *mcp.CallToolRequest, input createPRInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CreatePullRequest(ctx, input), nil
	})
	mcp.AddTool(server, defs[21], func(ctx context.Context, req *mcp.CallToolRequest, input commentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.AddPullRequestComment(ctx, input), nil
	})
	mcp.AddTool(server, defs[22], func(ctx context.Context, req *mcp.CallToolRequest, input reviewStatusInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.SetPullRequestReviewStatus(ctx, input), nil
	})
	mcp.AddTool(server, defs[23], func(ctx context.Context, req *mcp.CallToolRequest, input transitionInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.MergePullRequest(ctx, input), nil
	})
	mcp.AddTool(server, defs[24], func(ctx context.Context, req *mcp.CallToolRequest, input transitionInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.DeclinePullRequest(ctx, input), nil
	})
	mcp.AddTool(server, defs[25], func(ctx context.Context, req *mcp.CallToolRequest, input transitionInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.ReopenPullRequest(ctx, input), nil
	})
}
