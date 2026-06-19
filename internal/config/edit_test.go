package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteKeys_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	orig := `# my project secrets
project = "demo"   # namespace
profile = "branch"

keys = ["A", "B"]
# shims are opt-in
shims = ["forge"]
`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteKeys(dir, []string{"B", "A", "C"}); err != nil {
		t.Fatalf("WriteKeys: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dir, FileName))
	s := string(got)

	for _, want := range []string{
		"# my project secrets",
		`project = "demo"   # namespace`,
		"# shims are opt-in",
		`shims = ["forge"]`,
		`keys = ["A", "B", "C"]`, // merged + sorted
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected output to contain %q\n--- got ---\n%s", want, s)
		}
	}
}

func TestWriteKeys_AppendsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("profile = \"branch\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteKeys(dir, []string{"X"}); err != nil {
		t.Fatalf("WriteKeys: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, FileName))
	if !strings.Contains(string(got), `keys = ["X"]`) {
		t.Errorf("expected appended keys array, got:\n%s", got)
	}
}
