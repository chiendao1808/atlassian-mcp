package tools

import (
	"context"

	"github.com/chiendao1808/atlassian-mcp/internal/observability"
	"github.com/chiendao1808/atlassian-mcp/internal/result"
)

// ListSpacesInput filters Confluence's visible space collection.
type ListSpacesInput struct {
	SpaceKey string `json:"spaceKey,omitempty" jsonschema:"Optional Confluence space key filter"`
	Type     string `json:"type,omitempty" jsonschema:"Optional space type: global or personal"`
	Status   string `json:"status,omitempty" jsonschema:"Optional space status: current or archived"`
	Label    string `json:"label,omitempty" jsonschema:"Optional label filter"`
	Expand   string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value"`
	Start    *int   `json:"start,omitempty" jsonschema:"Optional non-negative start offset"`
	Limit    *int   `json:"limit,omitempty" jsonschema:"Optional positive limit; defaults to 25 when omitted"`
}

// GetSpaceInput selects one Confluence space by key.
type GetSpaceInput struct {
	SpaceKey string `json:"spaceKey" jsonschema:"Confluence space key (required)"`
	Expand   string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value"`
}

// ListSpaceContentInput selects the grouped content view for one Confluence space.
type ListSpaceContentInput struct {
	SpaceKey string `json:"spaceKey" jsonschema:"Confluence space key (required)"`
	Depth    string `json:"depth,omitempty" jsonschema:"Optional depth: all or root; omitted uses Confluence default all"`
	Expand   string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value"`
	Start    *int   `json:"start,omitempty" jsonschema:"Optional non-negative start offset"`
	Limit    *int   `json:"limit,omitempty" jsonschema:"Optional positive limit; defaults to 25 when omitted"`
}

// ListSpaces retrieves visible Confluence spaces with documented filters.
func (s *Service) ListSpaces(ctx context.Context, input ListSpacesInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_list_spaces")
	if errEnv != nil {
		return *errEnv
	}
	if input.Type != "" && input.Type != "global" && input.Type != "personal" {
		return result.Fail("confluence", "confluence_list_spaces", "VALIDATION_ERROR", "type must be global or personal")
	}
	if input.Status != "" && input.Status != "current" && input.Status != "archived" {
		return result.Fail("confluence", "confluence_list_spaces", "VALIDATION_ERROR", "status must be current or archived")
	}
	limit, invalid := validatedPage("confluence_list_spaces", input.Start, input.Limit, 25)
	if invalid != nil {
		return *invalid
	}
	query := q(
		"spaceKey", input.SpaceKey,
		"type", input.Type,
		"status", input.Status,
		"label", input.Label,
		"expand", input.Expand,
	)
	query = query.int("start", input.Start).intValue("limit", limit)
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/space", query, &out); err != nil {
		return confluenceClientError("confluence_list_spaces", "", err)
	}
	return result.OK("confluence", "confluence_list_spaces", observability.Redact(out))
}

// GetSpace retrieves one Confluence space by key after authentication.
func (s *Service) GetSpace(ctx context.Context, input GetSpaceInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_get_space")
	if errEnv != nil {
		return *errEnv
	}
	spaceKey, invalid := cleanPathSegment("confluence_get_space", "spaceKey", input.SpaceKey)
	if invalid != nil {
		return *invalid
	}
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/space/"+spaceKey, q("expand", input.Expand), &out); err != nil {
		return confluenceClientError("confluence_get_space", "", err)
	}
	return result.OK("confluence", "confluence_get_space", observability.Redact(out))
}

// ListSpaceContent retrieves Confluence's grouped content view for one space.
func (s *Service) ListSpaceContent(ctx context.Context, input ListSpaceContentInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_list_space_content")
	if errEnv != nil {
		return *errEnv
	}
	spaceKey, invalid := cleanPathSegment("confluence_list_space_content", "spaceKey", input.SpaceKey)
	if invalid != nil {
		return *invalid
	}
	if input.Depth != "" && input.Depth != "all" && input.Depth != "root" {
		return result.Fail("confluence", "confluence_list_space_content", "VALIDATION_ERROR", "depth must be all or root")
	}
	limit, invalid := validatedPage("confluence_list_space_content", input.Start, input.Limit, 25)
	if invalid != nil {
		return *invalid
	}
	query := q("depth", input.Depth, "expand", input.Expand)
	query = query.int("start", input.Start).intValue("limit", limit)
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/space/"+spaceKey+"/content", query, &out); err != nil {
		return confluenceClientError("confluence_list_space_content", "", err)
	}
	return result.OK("confluence", "confluence_list_space_content", observability.Redact(out))
}
