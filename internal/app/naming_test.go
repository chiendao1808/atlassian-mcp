package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForbiddenCredentialEnvironmentNamesStayOutOfImplementation(t *testing.T) {
	forbidden := []string{
		"JIRA_" + "USERNAME",
		"JIRA_" + "USER",
		"JIRA_" + "PASSWORD",
		"JIRA_" + "TOKEN",
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
			if strings.Contains(text, name) {
				t.Fatalf("forbidden Jira credential env name %s found in %s", name, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
