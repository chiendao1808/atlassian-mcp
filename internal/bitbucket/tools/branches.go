package tools

import (
	"context"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/result"
)

type repositoryInput struct {
	RepositorySlug string `json:"repositorySlug"`
}

type listBranchesInput struct {
	RepositorySlug string `json:"repositorySlug"`
	FilterText     string `json:"filterText,omitempty"`
	Base           string `json:"base,omitempty"`
	Details        *bool  `json:"details,omitempty"`
	OrderBy        string `json:"orderBy,omitempty"`
	Start          *int   `json:"start,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
}

type createBranchInput struct {
	RepositorySlug string `json:"repositorySlug"`
	Name           string `json:"name"`
	StartPoint     string `json:"startPoint"`
	Message        string `json:"message,omitempty"`
}

func (s *Service) GetRepository(ctx context.Context, in repositoryInput) result.Envelope {
	return s.getJSON(ctx, "bitbucket_get_repository", in.RepositorySlug, "", nil, "repository")
}

func (s *Service) ListBranches(ctx context.Context, in listBranchesInput) result.Envelope {
	if in.OrderBy != "" && in.OrderBy != "ALPHABETICAL" && in.OrderBy != "MODIFICATION" {
		return fail("bitbucket_list_branches", "orderBy must be ALPHABETICAL or MODIFICATION")
	}
	return s.getJSON(ctx, "bitbucket_list_branches", in.RepositorySlug, "branches", q(
		"filterText", in.FilterText, "base", in.Base, "orderBy", in.OrderBy,
	).bool("details", in.Details).page(in.Start, in.Limit), "branches")
}

func (s *Service) GetDefaultBranch(ctx context.Context, in repositoryInput) result.Envelope {
	return s.getJSON(ctx, "bitbucket_get_default_branch", in.RepositorySlug, "branches/default", nil, "branch")
}

func (s *Service) CreateBranch(ctx context.Context, in createBranchInput) result.Envelope {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.StartPoint) == "" {
		return fail("bitbucket_create_branch", "name and startPoint are required")
	}
	body := map[string]any{"name": in.Name, "startPoint": in.StartPoint}
	if in.Message != "" {
		body["message"] = in.Message
	}
	return s.postJSON(ctx, "bitbucket_create_branch", in.RepositorySlug, "branches", nil, body, "branch")
}
