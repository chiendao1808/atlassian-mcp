package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/auth"
)

func TestClientBuildsContextPathURLAndAddsBasicAuth(t *testing.T) {
	var seenPath, seenQuery, seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenQuery = r.URL.RawQuery
		seenAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "12345"})
	}))
	t.Cleanup(server.Close)

	c := New(server.URL+"/confluence", server.Client(), 1<<20)
	var out map[string]any
	err := c.GetJSON(context.Background(), auth.NewCredential("alice", "secret"), "/content/12345", map[string][]string{"expand": {"body.storage,version"}}, &out)
	if err != nil {
		t.Fatalf("GetJSON error = %v", err)
	}
	if seenPath != "/confluence/rest/api/content/12345" || seenQuery != "expand=body.storage%2Cversion" {
		t.Fatalf("path=%q query=%q", seenPath, seenQuery)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q", seenAuth)
	}
}

func TestClientMapsNotFoundAsNotFoundOrNotVisible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c := New(server.URL, server.Client(), 1<<20)
	var out map[string]any
	err := c.GetJSON(context.Background(), auth.NewCredential("alice", "secret"), "/content/12345", nil, &out)
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("err=%T %v", err, err)
	}
	if httpErr.Code != "NOT_FOUND_OR_NOT_VISIBLE" || strings.Contains(strings.ToLower(httpErr.Error()), "secret") {
		t.Fatalf("httpErr=%+v", httpErr)
	}
}
