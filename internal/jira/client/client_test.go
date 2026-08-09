package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
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
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1"})
	}))
	t.Cleanup(server.Close)

	c := New(server.URL+"/jira", server.Client(), 1<<20)
	var out map[string]any
	err := c.GetJSON(context.Background(), auth.NewCredential("alice", "secret"), "/issue/PROJ-1", map[string][]string{"fields": {"summary,status"}}, &out)
	if err != nil {
		t.Fatalf("GetJSON error = %v", err)
	}
	if seenPath != "/jira/rest/api/2/issue/PROJ-1" || seenQuery != "fields=summary%2Cstatus" {
		t.Fatalf("path=%q query=%q", seenPath, seenQuery)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q", seenAuth)
	}
}

func TestClientMapsJiraAndProxyErrorsWithoutLeakingAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["bad password"],"errors":{"password":"sentinel-secret"}}`))
	}))
	t.Cleanup(server.Close)

	c := New(server.URL, server.Client(), 1<<20)
	var out map[string]any
	err := c.GetJSON(context.Background(), auth.NewCredential("alice", "sentinel-secret"), "/myself", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(strings.ToLower(err.Error()), "sentinel-secret") || strings.Contains(strings.ToLower(err.Error()), "authorization") {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func TestClientDeleteJSONSendsQueryAndHandlesNoContent(t *testing.T) {
	var seenMethod, seenPath, seenQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	c := New(server.URL, server.Client(), 1<<20)
	err := c.DeleteJSON(context.Background(), auth.NewCredential("alice", "secret"), "/issue/PROJ-1", map[string][]string{"deleteSubtasks": {"true"}}, nil)
	if err != nil {
		t.Fatalf("DeleteJSON error = %v", err)
	}
	if seenMethod != http.MethodDelete || seenPath != "/rest/api/2/issue/PROJ-1" || seenQuery != "deleteSubtasks=true" {
		t.Fatalf("method=%q path=%q query=%q", seenMethod, seenPath, seenQuery)
	}
}

func TestClientPostJSONQuerySendsQueryAndJSONBody(t *testing.T) {
	var seenMethod, seenQuery string
	var seenBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenQuery = r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&seenBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001"})
	}))
	t.Cleanup(server.Close)

	c := New(server.URL, server.Client(), 1<<20)
	var out map[string]any
	err := c.PostJSONQuery(context.Background(), auth.NewCredential("alice", "secret"), "/issue/PROJ-1/worklog",
		map[string][]string{"adjustEstimate": {"new"}, "newEstimate": {"2d"}},
		map[string]any{"timeSpentSeconds": 3600}, &out)
	if err != nil {
		t.Fatalf("PostJSONQuery error = %v", err)
	}
	if seenMethod != http.MethodPost || seenQuery != "adjustEstimate=new&newEstimate=2d" {
		t.Fatalf("method=%q query=%q", seenMethod, seenQuery)
	}
	if seenBody["timeSpentSeconds"] != float64(3600) {
		t.Fatalf("body=%+v", seenBody)
	}
	if out["id"] != "10001" {
		t.Fatalf("out=%+v", out)
	}
}

func TestClientDoMultipartSendsFieldsFileAndExtraHeaders(t *testing.T) {
	var seenContentType, seenToken, seenFileName, seenFileContent, seenFormValue string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenContentType = r.Header.Get("Content-Type")
		seenToken = r.Header.Get("X-Atlassian-Token")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		seenFormValue = r.FormValue("description")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		seenFileName = header.Filename
		b, _ := io.ReadAll(file)
		seenFileContent = string(b)
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "10000", "filename": "notes.txt"}})
	}))
	t.Cleanup(server.Close)

	c := New(server.URL, server.Client(), 1<<20)
	var out []map[string]any
	err := c.DoMultipart(context.Background(), auth.NewCredential("alice", "secret"), "/issue/PROJ-1/attachments",
		map[string]string{"description": "upload"}, "file", "notes.txt", strings.NewReader("hello world"),
		map[string]string{"X-Atlassian-Token": "nocheck"}, &out)
	if err != nil {
		t.Fatalf("DoMultipart error = %v", err)
	}
	if !strings.HasPrefix(seenContentType, "multipart/form-data") {
		t.Fatalf("content type = %q", seenContentType)
	}
	if seenToken != "nocheck" {
		t.Fatalf("X-Atlassian-Token = %q", seenToken)
	}
	if seenFileName != "notes.txt" || seenFileContent != "hello world" {
		t.Fatalf("file=%q content=%q", seenFileName, seenFileContent)
	}
	if seenFormValue != "upload" {
		t.Fatalf("form value = %q", seenFormValue)
	}
	if len(out) != 1 || out[0]["id"] != "10000" {
		t.Fatalf("out=%+v", out)
	}
}
