package tools

import (
	"context"
	"strings"
	"time"

	"github.com/chiendao1808/atlassian-mcp/internal/observability"
	"github.com/chiendao1808/atlassian-mcp/internal/result"
)

// SearchContentInput selects Confluence content with raw CQL and optional passthrough expansions.
type SearchContentInput struct {
	CQL        string `json:"cql" jsonschema:"Raw Confluence CQL query (required)"`
	CQLContext string `json:"cqlcontext,omitempty" jsonschema:"Optional Confluence cqlcontext query value"`
	Expand     string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value"`
	Start      *int   `json:"start,omitempty" jsonschema:"Optional non-negative start offset"`
	Limit      *int   `json:"limit,omitempty" jsonschema:"Optional positive limit; defaults to 25 when omitted"`
}

// GetContentInput selects one Confluence content item and optional native query parameters.
type GetContentInput struct {
	ContentID string `json:"contentId" jsonschema:"Confluence content ID (required)"`
	Status    string `json:"status,omitempty" jsonschema:"Optional Confluence content status"`
	Version   *int   `json:"version,omitempty" jsonschema:"Optional positive content version"`
	Expand    string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value"`
}

// ListContentInput filters Confluence's /content collection without changing its native shape.
type ListContentInput struct {
	Type       string `json:"type,omitempty" jsonschema:"Optional content type: page or blogpost"`
	SpaceKey   string `json:"spaceKey,omitempty" jsonschema:"Optional Confluence space key filter"`
	Title      string `json:"title,omitempty" jsonschema:"Optional title filter"`
	Status     string `json:"status,omitempty" jsonschema:"Optional Confluence content status"`
	PostingDay string `json:"postingDay,omitempty" jsonschema:"Optional blog posting day in yyyy-mm-dd format"`
	Expand     string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value"`
	Start      *int   `json:"start,omitempty" jsonschema:"Optional non-negative start offset"`
	Limit      *int   `json:"limit,omitempty" jsonschema:"Optional positive limit; defaults to 25 when omitted"`
}

// ListContentPropertiesInput selects properties for one content item.
type ListContentPropertiesInput struct {
	ContentID string `json:"contentId" jsonschema:"Confluence content ID (required)"`
	Expand    string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value"`
	Start     *int   `json:"start,omitempty" jsonschema:"Optional non-negative start offset"`
	Limit     *int   `json:"limit,omitempty" jsonschema:"Optional positive limit; defaults to 10 when omitted"`
}

// GetContentPropertyInput selects one content property by content ID and property key.
type GetContentPropertyInput struct {
	ContentID string `json:"contentId" jsonschema:"Confluence content ID (required)"`
	Key       string `json:"key" jsonschema:"Content property key (required)"`
	Expand    string `json:"expand,omitempty" jsonschema:"Optional comma-separated Confluence expand value"`
}

// SearchContent runs a raw CQL content search after authentication.
func (s *Service) SearchContent(ctx context.Context, input SearchContentInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_search_content")
	if errEnv != nil {
		return *errEnv
	}
	if strings.TrimSpace(input.CQL) == "" {
		return result.Fail("confluence", "confluence_search_content", "VALIDATION_ERROR", "cql is required")
	}
	limit, invalid := validatedPage("confluence_search_content", input.Start, input.Limit, 25)
	if invalid != nil {
		return *invalid
	}
	query := q("cql", input.CQL, "cqlcontext", input.CQLContext, "expand", input.Expand)
	query = query.int("start", input.Start).intValue("limit", limit)
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/content/search", query, &out); err != nil {
		return confluenceClientError("confluence_search_content", "", err)
	}
	return result.OK("confluence", "confluence_search_content", observability.Redact(out))
}

// GetContent retrieves one Confluence content item by ID after authentication.
func (s *Service) GetContent(ctx context.Context, input GetContentInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_get_content")
	if errEnv != nil {
		return *errEnv
	}
	contentID, invalid := cleanPathSegment("confluence_get_content", "contentId", input.ContentID)
	if invalid != nil {
		return *invalid
	}
	if input.Version != nil && *input.Version <= 0 {
		return result.Fail("confluence", "confluence_get_content", "VALIDATION_ERROR", "version must be positive")
	}
	query := q("status", input.Status, "expand", input.Expand)
	query = query.int("version", input.Version)
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/content/"+contentID, query, &out); err != nil {
		return confluenceClientError("confluence_get_content", "", err)
	}
	return result.OK("confluence", "confluence_get_content", observability.Redact(out))
}

// ListContent retrieves Confluence content with the documented collection filters.
func (s *Service) ListContent(ctx context.Context, input ListContentInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_list_content")
	if errEnv != nil {
		return *errEnv
	}
	if input.Type != "" && input.Type != "page" && input.Type != "blogpost" {
		return result.Fail("confluence", "confluence_list_content", "VALIDATION_ERROR", "type must be page or blogpost")
	}
	if input.PostingDay != "" {
		if _, err := time.Parse("2006-01-02", input.PostingDay); err != nil {
			return result.Fail("confluence", "confluence_list_content", "VALIDATION_ERROR", "postingDay must use yyyy-mm-dd")
		}
	}
	limit, invalid := validatedPage("confluence_list_content", input.Start, input.Limit, 25)
	if invalid != nil {
		return *invalid
	}
	query := q(
		"type", input.Type,
		"spaceKey", input.SpaceKey,
		"title", input.Title,
		"status", input.Status,
		"postingDay", input.PostingDay,
		"expand", input.Expand,
	)
	query = query.int("start", input.Start).intValue("limit", limit)
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/content", query, &out); err != nil {
		return confluenceClientError("confluence_list_content", "", err)
	}
	return result.OK("confluence", "confluence_list_content", observability.Redact(out))
}

// ListContentProperties retrieves property objects for one content item.
func (s *Service) ListContentProperties(ctx context.Context, input ListContentPropertiesInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_list_content_properties")
	if errEnv != nil {
		return *errEnv
	}
	contentID, invalid := cleanPathSegment("confluence_list_content_properties", "contentId", input.ContentID)
	if invalid != nil {
		return *invalid
	}
	limit, invalid := validatedPage("confluence_list_content_properties", input.Start, input.Limit, 10)
	if invalid != nil {
		return *invalid
	}
	query := q("expand", input.Expand).int("start", input.Start).intValue("limit", limit)
	var out map[string]any
	if err := s.client.GetJSON(ctx, cred, "/content/"+contentID+"/property", query, &out); err != nil {
		return confluenceClientError("confluence_list_content_properties", "", err)
	}
	return result.OK("confluence", "confluence_list_content_properties", observability.Redact(out))
}

// GetContentProperty retrieves one native content property object by key.
func (s *Service) GetContentProperty(ctx context.Context, input GetContentPropertyInput) result.Envelope {
	cred, errEnv := s.requireCredential("confluence_get_content_property")
	if errEnv != nil {
		return *errEnv
	}
	contentID, invalid := cleanPathSegment("confluence_get_content_property", "contentId", input.ContentID)
	if invalid != nil {
		return *invalid
	}
	key, invalid := cleanPathSegment("confluence_get_content_property", "key", input.Key)
	if invalid != nil {
		return *invalid
	}
	var out map[string]any
	if err := s.client.GetJSON(
		ctx,
		cred,
		"/content/"+contentID+"/property/"+key,
		q("expand", input.Expand),
		&out,
	); err != nil {
		return confluenceClientError("confluence_get_content_property", "", err)
	}
	return result.OK("confluence", "confluence_get_content_property", observability.Redact(out))
}
