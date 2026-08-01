package client

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultMaxResponseBytes int64 = 10 << 20

type Client struct {
	baseURL          *url.URL
	projectKey       string
	token            string
	httpClient       *http.Client
	maxResponseBytes int64
	options          Options
}

type Options struct {
	Stderr         interface{ Write([]byte) (int, error) }
	RetryBaseDelay time.Duration
}

type Endpoint struct {
	Path     string
	Template string
}

func New(base, projectKey, token string, hc *http.Client, maxResponseBytes int64, options Options) *Client {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		u = &url.URL{}
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}
	if options.RetryBaseDelay <= 0 {
		options.RetryBaseDelay = 25 * time.Millisecond
	}
	return &Client{baseURL: u, projectKey: projectKey, token: token, httpClient: hc, maxResponseBytes: maxResponseBytes, options: options}
}

func (c *Client) RepositoryEndpoint(repositorySlug, template string, segments ...string) (Endpoint, error) {
	pathParts := []string{"projects", c.projectKey, "repos", repositorySlug}
	pathParts = append(pathParts, segments...)
	p, err := encodedPath(pathParts...)
	if err != nil {
		return Endpoint{}, err
	}
	if template == "" {
		template = p
	}
	return Endpoint{Path: p, Template: template}, nil
}

func (c *Client) RepositoryFileEndpoint(repositorySlug, template, endpointName, filePath string) (Endpoint, error) {
	fileSegments, err := cleanFilePath(filePath)
	if err != nil {
		return Endpoint{}, err
	}
	pathParts := []string{"projects", c.projectKey, "repos", repositorySlug, endpointName}
	pathParts = append(pathParts, fileSegments...)
	p, err := encodedPath(pathParts...)
	if err != nil {
		return Endpoint{}, err
	}
	if template == "" {
		template = p
	}
	return Endpoint{Path: p, Template: template}, nil
}

func encodedPath(parts ...string) (string, error) {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.ContainsRune(part, 0) || part == "" {
			return "", errInvalidPath
		}
		out = append(out, url.PathEscape(part))
	}
	return "/" + strings.Join(out, "/"), nil
}

func cleanFilePath(value string) ([]string, error) {
	if value == "" || strings.ContainsRune(value, 0) {
		return nil, errInvalidPath
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == ".." {
			return nil, errInvalidPath
		}
	}
	return parts, nil
}

func (c *Client) urlFor(endpoint Endpoint, query map[string][]string) string {
	u := *c.baseURL
	escaped := strings.TrimRight(c.baseURL.EscapedPath(), "/") + "/rest/api/1.0" + endpoint.Path
	unescaped, err := url.PathUnescape(escaped)
	if err == nil {
		u.Path = unescaped
		u.RawPath = escaped
	} else {
		u.Path = strings.TrimRight(c.baseURL.Path, "/") + "/rest/api/1.0" + endpoint.Path
	}
	q := url.Values{}
	for key, values := range query {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
