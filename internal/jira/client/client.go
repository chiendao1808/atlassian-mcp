package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/observability"
)

type Client struct {
	baseURL          *url.URL
	httpClient       *http.Client
	maxResponseBytes int64
}

type HTTPError struct {
	StatusCode int
	Code       string
	Message    string
	Detail     any
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func New(base string, hc *http.Client, maxResponseBytes int64) *Client {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		u = &url.URL{}
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	if maxResponseBytes <= 0 {
		maxResponseBytes = 10 << 20
	}
	return &Client{baseURL: u, httpClient: hc, maxResponseBytes: maxResponseBytes}
}

// GetJSON issues a GET request with optional query parameters and decodes a JSON response into out.
func (c *Client) GetJSON(ctx context.Context, cred auth.Credential, apiPath string, query map[string][]string, out any) error {
	return c.doJSON(ctx, cred, http.MethodGet, apiPath, query, nil, out)
}

// PostJSON issues a POST request with a JSON-marshaled body and decodes a JSON response into out.
func (c *Client) PostJSON(ctx context.Context, cred auth.Credential, apiPath string, body any, out any) error {
	return c.doJSON(ctx, cred, http.MethodPost, apiPath, nil, body, out)
}

// PutJSON issues a PUT request with a JSON-marshaled body and decodes a JSON response into out.
func (c *Client) PutJSON(ctx context.Context, cred auth.Credential, apiPath string, body any, out any) error {
	return c.doJSON(ctx, cred, http.MethodPut, apiPath, nil, body, out)
}

// DeleteJSON issues a DELETE request with optional query parameters (no body) and decodes a JSON
// response into out when Jira returns one. Query support exists for delete variants that Jira
// gates by a query flag or selector (e.g. deleteSubtasks, watcher-removal username) rather than a body.
func (c *Client) DeleteJSON(ctx context.Context, cred auth.Credential, apiPath string, query map[string][]string, out any) error {
	return c.do(ctx, cred, http.MethodDelete, apiPath, query, "", nil, nil, out)
}

// PostJSONQuery issues a POST request carrying both a JSON-marshaled body and query parameters,
// decoding a JSON response into out. Kept separate from PostJSON so the common no-query call site
// is unaffected; needed by endpoints (e.g. worklog adjustEstimate) that require both simultaneously.
func (c *Client) PostJSONQuery(ctx context.Context, cred auth.Credential, apiPath string, query map[string][]string, body any, out any) error {
	return c.doJSON(ctx, cred, http.MethodPost, apiPath, query, body, out)
}

// DoMultipart POSTs a multipart/form-data request built from fields plus one file part
// (fileField/fileName/content), applies extraHeaders (e.g. Jira's mandatory
// X-Atlassian-Token: nocheck XSRF bypass for uploads), and decodes a JSON response into out.
func (c *Client) DoMultipart(ctx context.Context, cred auth.Credential, apiPath string, fields map[string]string, fileField, fileName string, content io.Reader, extraHeaders map[string]string, out any) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for key, value := range fields {
		if err := w.WriteField(key, value); err != nil {
			return err
		}
	}
	part, err := w.CreateFormFile(fileField, fileName)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, content); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.do(ctx, cred, http.MethodPost, apiPath, nil, w.FormDataContentType(), extraHeaders, &buf, out)
}

// doJSON marshals body (when non-nil) to a JSON request payload and delegates to do, setting the
// application/json content type only when a body is actually being sent. GetJSON/PostJSON/PutJSON
// are thin wrappers over this; their signatures and behavior are unchanged by this refactor.
func (c *Client) doJSON(ctx context.Context, cred auth.Credential, method, apiPath string, query map[string][]string, body any, out any) error {
	var rdr io.Reader
	contentType := ""
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
		contentType = "application/json"
	}
	return c.do(ctx, cred, method, apiPath, query, contentType, nil, rdr, out)
}

// do is the single low-level request/response path shared by every Jira client entry point: it
// applies Basic Auth, Accept: application/json, an optional Content-Type, and any extraHeaders,
// then runs the response tail that previously lived only in doJSON — 204 short-circuit, a
// maxResponseBytes-limited read, size-check, mapHTTPError on non-2xx, and JSON unmarshal into out.
// This tail is byte-for-byte equivalent to the pre-refactor doJSON behavior; no retry (parity with
// the existing Jira client, which never retried).
func (c *Client) do(ctx context.Context, cred auth.Credential, method, apiPath string, query map[string][]string, contentType string, extraHeaders map[string]string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.urlFor(apiPath, query), body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(cred.Username(), cred.Password())
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return &HTTPError{Code: "UPSTREAM_UNREACHABLE", Message: "Jira request failed"}
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return nil
	}
	limited := io.LimitReader(res.Body, c.maxResponseBytes+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return &HTTPError{StatusCode: res.StatusCode, Code: "UPSTREAM_SERVER_ERROR", Message: "Jira response could not be read"}
	}
	if int64(len(b)) > c.maxResponseBytes {
		return &HTTPError{StatusCode: res.StatusCode, Code: "RESPONSE_TOO_LARGE", Message: "Jira response exceeded ATLASSIAN_MAX_RESPONSE_BYTES"}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return mapHTTPError(res.StatusCode, b)
	}
	if out == nil || len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return &HTTPError{StatusCode: res.StatusCode, Code: "UPSTREAM_SERVER_ERROR", Message: "Jira response was not valid JSON"}
	}
	return nil
}

func (c *Client) urlFor(apiPath string, query map[string][]string) string {
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + "/rest/api/2/" + strings.TrimLeft(apiPath, "/")
	q := url.Values{}
	for key, values := range query {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func mapHTTPError(status int, body []byte) *HTTPError {
	code := mapStatusCode(status)
	var parsed map[string]any
	if json.Unmarshal(body, &parsed) == nil {
		return &HTTPError{StatusCode: status, Code: code, Message: "Jira returned an error", Detail: observability.Redact(parsed)}
	}
	return &HTTPError{StatusCode: status, Code: code, Message: "Jira returned a non-JSON error"}
}

func mapStatusCode(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "UPSTREAM_AUTHENTICATION_FAILED"
	case http.StatusForbidden:
		return "UPSTREAM_PERMISSION_DENIED"
	case http.StatusNotFound:
		return "UPSTREAM_NOT_FOUND"
	case http.StatusConflict:
		return "UPSTREAM_CONFLICT"
	case http.StatusTooManyRequests:
		return "UPSTREAM_RATE_LIMITED"
	default:
		if status >= 500 {
			return "UPSTREAM_SERVER_ERROR"
		}
		return "VALIDATION_ERROR"
	}
}
