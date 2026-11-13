package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestRequirementsDocumentReadsCommittedBytes(t *testing.T) {
	directory := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", directory}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.test")
	if err := os.MkdirAll(filepath.Join(directory, "onespec"), 0755); err != nil {
		t.Fatal(err)
	}
	content := "# Requirements\nUse staging.\n"
	file := filepath.Join(directory, "onespec", "prd.md")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "requirements")
	digest := sha256.Sum256([]byte(content))
	document := &workbench.RequirementsDocument{Content: content, Ref: workbench.OneSpecArtifactRef{RepositoryID: "repo", Path: "onespec/prd.md", Commit: git("rev-parse", "HEAD"), Digest: hex.EncodeToString(digest[:])}}
	ref, _ := json.Marshal(map[string]string{"local_path": directory})
	task := Task{ProjectResources: []ProjectResourceData{{ID: "repo", ResourceType: "local_directory", ResourceRef: ref}}}
	if err := verifyRequirementFile(t.Context(), document, task, directory); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("uncommitted edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifyRequirementFile(t.Context(), document, task, directory); err != nil {
		t.Fatal("uncommitted edit changed reviewed version", err)
	}
	document.Content = "fabricated content"
	digest = sha256.Sum256([]byte(document.Content))
	document.Ref.Digest = hex.EncodeToString(digest[:])
	if err := verifyRequirementFile(t.Context(), document, task, directory); err == nil {
		t.Fatal("accepted fabricated file with self-consistent digest")
	}
	document.CheckoutPath = t.TempDir()
	if err := verifyRequirementFile(t.Context(), document, task, directory); err == nil {
		t.Fatal("accepted unrelated checkout")
	}
}

func TestRequirementsDocumentRepositoryTransports(t *testing.T) {
	for _, remote := range []string{"https://example.test/team/repo.git", "https://user:credential@example.test/team/repo.git", "git@example.test:team/repo.git", "ssh://git@example.test/team/repo.git"} {
		if normalizeRequirementRepository(remote) != "example.test/team/repo" {
			t.Fatal("repository transport changed identity")
		}
	}
}
