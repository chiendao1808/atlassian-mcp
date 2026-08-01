package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type RawResponse struct {
	Body []byte
	Size int
}

func (c *Client) DoJSON(ctx context.Context, method string, endpoint Endpoint, query map[string][]string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	return c.do(ctx, method, endpoint, query, "application/json", rdr, func(payload []byte) error {
		if out == nil || len(bytes.TrimSpace(payload)) == 0 {
			return nil
		}
		if err := json.Unmarshal(payload, out); err != nil {
			return &HTTPError{Code: "UPSTREAM_SERVER_ERROR", Message: "Bitbucket response was not valid JSON"}
		}
		return nil
	})
}

func (c *Client) DoRaw(ctx context.Context, endpoint Endpoint, query map[string][]string) (RawResponse, error) {
	var raw RawResponse
	err := c.do(ctx, http.MethodGet, endpoint, query, "", nil, func(payload []byte) error {
		raw = RawResponse{Body: payload, Size: len(payload)}
		return nil
	})
	return raw, err
}

func (c *Client) DoMultipart(ctx context.Context, method string, endpoint Endpoint, fields map[string]string, fileField, fileName string, content io.Reader, out any) error {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
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
	return c.do(ctx, method, endpoint, nil, w.FormDataContentType(), &b, func(payload []byte) error {
		if out == nil || len(bytes.TrimSpace(payload)) == 0 {
			return nil
		}
		if err := json.Unmarshal(payload, out); err != nil {
			return &HTTPError{Code: "UPSTREAM_SERVER_ERROR", Message: "Bitbucket response was not valid JSON"}
		}
		return nil
	})
}

func (c *Client) do(ctx context.Context, method string, endpoint Endpoint, query map[string][]string, contentType string, body io.Reader, decode func([]byte) error) error {
	attempts := 1
	if isRead(method) && body == nil {
		attempts = 3
	}
	var last error
	for attempt := 1; attempt <= attempts; attempt++ {
		payload, retry, err := c.once(ctx, method, endpoint, query, contentType, body, decode)
		if err == nil {
			return nil
		}
		last = err
		if !retry || attempt == attempts {
			return err
		}
		_ = payload
		time.Sleep(c.options.RetryBaseDelay * time.Duration(attempt))
	}
	return last
}

func (c *Client) once(ctx context.Context, method string, endpoint Endpoint, query map[string][]string, contentType string, body io.Reader, decode func([]byte) error) ([]byte, bool, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, c.urlFor(endpoint, query), body)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, &HTTPError{Code: "UPSTREAM_TIMEOUT", Message: "Bitbucket request timed out or was cancelled"}
		}
		return nil, isRead(method), &HTTPError{Code: "UPSTREAM_UNREACHABLE", Message: "Bitbucket request failed"}
	}
	defer res.Body.Close()
	c.log(method, endpoint.Template, res.StatusCode, time.Since(start), res.Header.Get("X-Request-Id"))
	if res.StatusCode == http.StatusNoContent {
		return nil, false, nil
	}
	limited := io.LimitReader(res.Body, c.maxResponseBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, &HTTPError{StatusCode: res.StatusCode, Code: "UPSTREAM_SERVER_ERROR", Message: "Bitbucket response could not be read"}
	}
	if int64(len(payload)) > c.maxResponseBytes {
		return payload, false, &HTTPError{StatusCode: res.StatusCode, Code: "RESPONSE_TOO_LARGE", Message: "Bitbucket response exceeded ATLASSIAN_MAX_RESPONSE_BYTES"}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return payload, isRead(method) && (res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500), c.mapHTTPError(res.StatusCode, payload)
	}
	if err := decode(payload); err != nil {
		return payload, false, err
	}
	return payload, false, nil
}

func (c *Client) log(method, template string, status int, duration time.Duration, requestID string) {
	if c.options.Stderr == nil {
		return
	}
	line := fmt.Sprintf("bitbucket request method=%s path=%s status=%d duration_ms=%d", method, sanitizeToken(template, c.token), status, duration.Milliseconds())
	if requestID != "" {
		line += " request_id=" + sanitizeToken(requestID, c.token)
	}
	_, _ = fmt.Fprintln(c.options.Stderr, line)
}

func isRead(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func sanitizeToken(value, token string) string {
	if token == "" {
		return value
	}
	return strings.ReplaceAll(value, token, "[REDACTED]")
}
