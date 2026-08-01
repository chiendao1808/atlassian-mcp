package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const tokenSentinel = "BITBUCKET_TOKEN_SENTINEL"

func TestClientBuildsContextPathURLsAndAddsBearerHeader(t *testing.T) {
	var logs strings.Builder
	var gotAuth string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.EscapedPath()
		if r.URL.Query().Get("until") != "refs/heads/main" || r.URL.Query().Get("path") != "dir with space/uni-雪.txt" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("X-Request-Id", "req-123")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	c := New(server.URL+"/bitbucket", "~alice", tokenSentinel, server.Client(), 1024, Options{Stderr: &logs, RetryBaseDelay: time.Nanosecond})
	endpoint, err := c.RepositoryEndpoint("repo slug", "/projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}", "commits", "refs/heads/feature/a")
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	err = c.DoJSON(context.Background(), http.MethodGet, endpoint, map[string][]string{
		"until": {"refs/heads/main"},
		"path":  {"dir with space/uni-雪.txt"},
	}, nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer "+tokenSentinel {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	wantPath := "/bitbucket/rest/api/1.0/projects/~alice/repos/repo%20slug/commits/refs%2Fheads%2Ffeature%2Fa"
	if gotPath != wantPath {
		t.Fatalf("path = %s, want %s", gotPath, wantPath)
	}
	if strings.Contains(logs.String(), tokenSentinel) || !strings.Contains(logs.String(), "path=/projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}") || !strings.Contains(logs.String(), "request_id=req-123") {
		t.Fatalf("unexpected sanitized log: %q", logs.String())
	}
}

func TestClientFilePathBuilderEncodesSegmentsAndRejectsTraversal(t *testing.T) {
	c := New("https://bitbucket.internal.example.com/bitbucket", "PRJ", "token", nil, 1024, Options{})
	endpoint, err := c.RepositoryFileEndpoint("repo", "/projects/{projectKey}/repos/{repositorySlug}/raw/{path}", "raw", "dir with space/uni-雪.txt")
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Path != "/projects/PRJ/repos/repo/raw/dir%20with%20space/uni-%E9%9B%AA.txt" {
		t.Fatalf("file path = %s", endpoint.Path)
	}
	for _, path := range []string{"", "../secret.txt", "dir/../secret.txt", "bad\x00path"} {
		if _, err := c.RepositoryFileEndpoint("repo", "template", "raw", path); err == nil {
			t.Fatalf("path %q should be rejected", path)
		}
	}
}

func TestClientMapsBitbucketErrorsAndSanitizesDetails(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "bitbucket", "errors", "bitbucket-errors.json"))
	if err != nil {
		t.Fatal(err)
	}
	for status, code := range map[int]string{
		400: "VALIDATION_ERROR",
		401: "UPSTREAM_AUTHENTICATION_FAILED",
		403: "UPSTREAM_PERMISSION_DENIED",
		404: "UPSTREAM_NOT_FOUND",
		409: "UPSTREAM_CONFLICT",
		415: "VALIDATION_ERROR",
		429: "UPSTREAM_RATE_LIMITED",
		500: "UPSTREAM_SERVER_ERROR",
	} {
		t.Run(code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write(body)
			}))
			defer server.Close()
			c := New(server.URL, "PRJ", tokenSentinel, server.Client(), 4096, Options{RetryBaseDelay: time.Nanosecond})
			err := c.DoJSON(context.Background(), http.MethodPost, Endpoint{Path: "/projects/PRJ/repos/repo", Template: "template"}, nil, map[string]any{"token": tokenSentinel}, nil)
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("err = %#v", err)
			}
			if httpErr.Code != code || httpErr.StatusCode != status {
				t.Fatalf("code/status = %s/%d", httpErr.Code, httpErr.StatusCode)
			}
			if strings.Contains(httpErr.Error(), tokenSentinel) || strings.Contains(fmt.Sprint(httpErr.Detail), tokenSentinel) {
				t.Fatalf("token leaked in error: %#v", httpErr)
			}
		})
	}
}

func TestClientHandlesNonJSONMalformedEmptyOversizedAndCancelledResponses(t *testing.T) {
	t.Run("html proxy error", func(t *testing.T) {
		body, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "bitbucket", "errors", "proxy.html"))
		if err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write(body)
		}))
		defer server.Close()
		err = New(server.URL, "PRJ", "token", server.Client(), 1024, Options{RetryBaseDelay: time.Nanosecond}).DoJSON(context.Background(), http.MethodPost, Endpoint{Path: "/x", Template: "x"}, nil, nil, nil)
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.Code != "UPSTREAM_SERVER_ERROR" {
			t.Fatalf("err = %#v", err)
		}
	})
	t.Run("malformed success json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("{"))
		}))
		defer server.Close()
		var out map[string]any
		err := New(server.URL, "PRJ", "token", server.Client(), 1024, Options{}).DoJSON(context.Background(), http.MethodGet, Endpoint{Path: "/x", Template: "x"}, nil, nil, &out)
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.Code != "UPSTREAM_SERVER_ERROR" {
			t.Fatalf("err = %#v", err)
		}
	})
	t.Run("empty 204", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		if err := New(server.URL, "PRJ", "token", server.Client(), 1024, Options{}).DoJSON(context.Background(), http.MethodPut, Endpoint{Path: "/x", Template: "x"}, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("oversized body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"too":"large"}`))
		}))
		defer server.Close()
		err := New(server.URL, "PRJ", "token", server.Client(), 4, Options{}).DoJSON(context.Background(), http.MethodGet, Endpoint{Path: "/x", Template: "x"}, nil, nil, nil)
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.Code != "RESPONSE_TOO_LARGE" {
			t.Fatalf("err = %#v", err)
		}
	})
	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := New("http://127.0.0.1", "PRJ", "token", http.DefaultClient, 1024, Options{}).DoJSON(ctx, http.MethodGet, Endpoint{Path: "/x", Template: "x"}, nil, nil, nil)
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.Code != "UPSTREAM_TIMEOUT" {
			t.Fatalf("err = %#v", err)
		}
	})
}

func TestClientRetriesOnlyReadRequests(t *testing.T) {
	for _, tc := range []struct {
		method string
		want   int
	}{
		{http.MethodGet, 3},
		{http.MethodPost, 1},
		{http.MethodPut, 1},
		{http.MethodDelete, 1},
	} {
		t.Run(tc.method, func(t *testing.T) {
			count := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				if count < 3 {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"errors":[{"message":"try again"}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()
			var out map[string]any
			_ = New(server.URL, "PRJ", "token", server.Client(), 1024, Options{RetryBaseDelay: time.Nanosecond}).DoJSON(context.Background(), tc.method, Endpoint{Path: "/x", Template: "x"}, nil, nil, &out)
			if count != tc.want {
				t.Fatalf("requests = %d, want %d", count, tc.want)
			}
		})
	}
}

func TestClientReturnsRawBodyAndSendsMultipart(t *testing.T) {
	t.Run("raw", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("raw file"))
		}))
		defer server.Close()
		got, err := New(server.URL, "PRJ", "token", server.Client(), 1024, Options{}).DoRaw(context.Background(), Endpoint{Path: "/raw", Template: "raw"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(got.Body) != "raw file" || got.Size != len("raw file") {
			t.Fatalf("raw = %+v", got)
		}
	})
	t.Run("multipart", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mediaType != "multipart/form-data" {
				t.Fatalf("content type = %q err=%v", r.Header.Get("Content-Type"), err)
			}
			if err := r.ParseMultipartForm(1024); err != nil {
				t.Fatal(err)
			}
			file, _, err := r.FormFile("content")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			b, _ := io.ReadAll(file)
			if r.FormValue("branch") != "main" || string(b) != "hello" {
				t.Fatalf("form branch=%q content=%q", r.FormValue("branch"), string(b))
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()
		var out map[string]any
		err := New(server.URL, "PRJ", "token", server.Client(), 1024, Options{}).DoMultipart(context.Background(), http.MethodPut, Endpoint{Path: "/browse/a.txt", Template: "browse"}, map[string]string{"branch": "main"}, "content", "a.txt", bytes.NewReader([]byte("hello")), &out)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestPageRetainsNextPageStart(t *testing.T) {
	page := Page[map[string]any]{Start: 10, Limit: 25, Size: 25, IsLastPage: false, NextPageStart: intPtr(37)}
	if page.NextStart() == nil || *page.NextStart() != 37 {
		t.Fatalf("next start = %v", page.NextStart())
	}
	query := page.NextQuery(map[string][]string{"start": {"10"}, "limit": {"25"}})
	if query["start"][0] != "37" {
		t.Fatalf("next query start = %s, want server-provided 37", query["start"][0])
	}
}

func intPtr(v int) *int { return &v }
