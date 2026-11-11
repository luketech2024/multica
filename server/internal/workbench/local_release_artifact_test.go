package workbench

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalReleaseBuildUploadAndRetry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX build and deployment scripts")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("MULTICA_RELEASE_GIT_CREDENTIAL_HELPER", "")
	t.Setenv("DATABASE_URL", "must-not-reach-build")
	source := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = source
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git fixture: %v %s", err, output)
		}
		return strings.TrimSpace(string(output))
	}
	run("init")
	run("config", "user.email", "fixture@example.test")
	run("config", "user.name", "Fixture")
	if err := os.WriteFile(filepath.Join(source, "index.html"), []byte("first version"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "fixture")
	run("tag", "v1")
	commit := run("rev-parse", "HEAD")
	app := DeploymentApplication{ID: deploymentTestID, SourceKind: "git", Source: source, ArtifactPath: "dist", BuildCommand: "test -z \"${DATABASE_URL:-}\"; mkdir -p dist; cp index.html dist/index.html"}
	change := ReleaseChange{ID: deploymentTestNode, Type: "app", Version: "v1"}
	artifact, err := PrepareLocalReleaseArtifact(context.Background(), deploymentTestID, app, change)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Commit != commit || artifact.Digest == "" {
		t.Fatalf("artifact not pinned: %+v", artifact)
	}
	if err := os.WriteFile(filepath.Join(source, "index.html"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "changed")
	run("tag", "-f", "v1")
	change.ResolvedVersion, change.ArtifactDigest = artifact.Commit, artifact.Digest
	retry, err := PrepareLocalReleaseArtifact(context.Background(), deploymentTestID, app, change)
	if err != nil || retry.Digest != artifact.Digest {
		t.Fatalf("retry changed artifact: %v", err)
	}
	deploy := filepath.Join(t.TempDir(), "www", "site")
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.Open(artifact.File)
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command("sh", "-ec", localArtifactUploadScript(deploy, deploymentTestID, artifact.Digest))
		command.Stdin = file
		output, err := command.CombinedOutput()
		file.Close()
		if err != nil {
			t.Fatalf("upload failed: %v %s", err, output)
		}
	}
	parentInfo, err := os.Stat(filepath.Dir(deploy))
	if err != nil || parentInfo.Mode().Perm()&0001 == 0 {
		t.Fatal("new website parent directory cannot be traversed by nginx")
	}
	content, err := os.ReadFile(filepath.Join(deploy, "releases", deploymentTestID, "index.html"))
	if err != nil || string(content) != "first version" {
		t.Fatalf("uploaded content changed: %v", err)
	}
	config := ApplicationDeployment{Kind: "nginx", Directory: deploy, Domain: "example.test", HTTPS: "off", HealthPath: "/"}
	script, err := ApplicationDeploymentScript(deploymentTestID, app, config, change, nil)
	if err != nil || strings.Contains(script, "git ") || strings.Contains(script, app.BuildCommand) {
		t.Fatalf("server still builds source: %v", err)
	}
	if err := os.WriteFile(artifact.File, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareLocalReleaseArtifact(context.Background(), deploymentTestID, app, change); err == nil {
		t.Fatal("accepted changed cached artifact")
	}
}

func TestLocalArtifactExcludesRepositoryMetadataAndRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".git", "node_modules"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "secret"), []byte("excluded"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("website"), 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "archive.tar.gz")
	if err := archiveReleaseArtifact(context.Background(), root, destination); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	header, err := reader.Next()
	if err != nil || header.Name != "index.html" {
		t.Fatal("repository metadata included")
	}
	if _, err := reader.Next(); err != io.EOF {
		t.Fatal("unexpected artifact entry")
	}
	if err := os.Symlink(destination, filepath.Join(root, "outside")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := archiveReleaseArtifact(context.Background(), root, filepath.Join(t.TempDir(), "bad.tar.gz")); err == nil {
		t.Fatal("accepted artifact symlink")
	}
}
