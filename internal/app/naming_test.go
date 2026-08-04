package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestForbiddenCredentialEnvironmentNamesStayOutOfImplementation guards the remaining forbidden
// alt-name/token env vars from SPECS.md Sec 4.2. JIRA_USERNAME/JIRA_PASSWORD were removed from this
// list by ADR-0004 (docs/decisions/0004-jira-credential-env-fallback.md), which allows
// jira_authenticate to fall back to them when the tool call omits username/password. Matching uses
// word boundaries so the still-forbidden JIRA_USER does not false-positive on JIRA_USERNAME.
func TestForbiddenCredentialEnvironmentNamesStayOutOfImplementation(t *testing.T) {
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`\bJIRA_USER\b`),
		regexp.MustCompile(`\bJIRA_TOKEN\b`),
	}
	skipDirs := map[string]bool{".git": true, ".agents": true, ".codex": true, ".harness-core": true}
	err := filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("../..", path)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)
		if d.IsDir() {
			if skipDirs[filepath.Base(path)] || rel == filepath.Clean("docs/specs") {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == filepath.Clean("internal/app/naming_test.go") || !(strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, ".md")) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)
		for _, name := range forbidden {
			if name.MatchString(text) {
				t.Fatalf("forbidden Jira credential env name %s found in %s", name, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
