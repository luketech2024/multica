package workbench

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/processtree"
)

type LocalReleaseArtifact struct {
	Commit string `json:"commit"`
	Digest string `json:"digest"`
	File   string `json:"-"`
}

func releaseCommand(ctx context.Context, directory, name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.Dir = directory
	// Builds receive user tooling configuration, without API/database credentials.
	for _, variable := range []string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "TEMP", "TMP", "TMPDIR", "LANG", "SSL_CERT_FILE", "SSL_CERT_DIR", "SSH_AUTH_SOCK", "WSL_INTEROP", "WSL_DISTRO_NAME", "WSLENV"} {
		if value, ok := os.LookupEnv(variable); ok {
			command.Env = append(command.Env, variable+"="+value)
		}
	}
	command.Env = append(command.Env, "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	output := &deploymentOutput{}
	command.Stdout = output
	command.Stderr = io.Discard
	if err := processtree.Run(ctx, command, 5*time.Second); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func localReleaseGit(ctx context.Context, directory string, args ...string) ([]byte, error) {
	if helper := os.Getenv("MULTICA_RELEASE_GIT_CREDENTIAL_HELPER"); helper != "" {
		args = append([]string{"-c", "credential.helper=", "-c", "credential.helper=" + helper}, args...)
	}
	return releaseCommand(ctx, directory, "git", args...)
}

// PrepareLocalReleaseArtifact reuses the same archive across retries and nodes.
func PrepareLocalReleaseArtifact(ctx context.Context, releaseID string, app DeploymentApplication, change ReleaseChange) (LocalReleaseArtifact, error) {
	var artifact LocalReleaseArtifact
	if !validDeploymentID(releaseID) || !validDeploymentID(change.ID) {
		return artifact, errors.New("invalid artifact identity")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return artifact, err
	}
	root := filepath.Join(cache, "multica", "release-artifacts", releaseID, change.ID)
	artifact.File = filepath.Join(root, "artifact.tar.gz")
	metadata := filepath.Join(root, "artifact.json")
	if raw, err := os.ReadFile(metadata); err == nil {
		if json.Unmarshal(raw, &artifact) != nil || artifact.Commit == "" || artifact.Digest == "" {
			return artifact, errors.New("local release artifact metadata is invalid")
		}
		digest, err := releaseArtifactDigest(artifact.File)
		if err != nil || digest != artifact.Digest || (change.ArtifactDigest != "" && change.ArtifactDigest != digest) || (change.ResolvedVersion != "" && change.ResolvedVersion != artifact.Commit) {
			return artifact, errors.New("local release artifact is missing or changed; restore the original artifact before retrying")
		}
		return artifact, nil
	}
	if change.ArtifactDigest != "" {
		return artifact, errors.New("local release artifact is unavailable; restore it before retrying")
	}
	version := change.ResolvedVersion
	if version == "" {
		version = change.Version
	}
	artifact.Commit, err = ResolveApplicationVersion(ctx, DeploymentResource{}, nil, app, version)
	if err != nil {
		return artifact, fmt.Errorf("local source resolution failed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return artifact, err
	}
	staging, err := os.MkdirTemp(filepath.Dir(root), ".build-")
	if err != nil {
		return artifact, err
	}
	defer os.RemoveAll(staging)
	source := filepath.Join(staging, "source")
	if _, err := localReleaseGit(ctx, "", "clone", "--no-checkout", "--", app.Source, source); err != nil {
		return artifact, errors.New("local repository checkout failed; check repository access on the build machine")
	}
	if _, err := localReleaseGit(ctx, source, "checkout", "--detach", artifact.Commit); err != nil {
		return artifact, errors.New("local checkout of the pinned commit failed")
	}
	if app.BuildCommand != "" {
		if _, err := releaseCommand(ctx, source, "sh", "-ec", app.BuildCommand); err != nil {
			return artifact, fmt.Errorf("local build failed: %w", err)
		}
	}
	artifactRoot, err := filepath.EvalSymlinks(filepath.Join(source, filepath.FromSlash(app.ArtifactPath)))
	if err != nil {
		return artifact, errors.New("local artifact directory does not exist; check the build command and artifact path")
	}
	relative, err := filepath.Rel(source, artifactRoot)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return artifact, errors.New("artifact directory must stay inside the checkout")
	}
	archive := filepath.Join(staging, "artifact.tar.gz")
	if err := archiveReleaseArtifact(ctx, artifactRoot, archive); err != nil {
		return artifact, err
	}
	artifact.Digest, err = releaseArtifactDigest(archive)
	if err != nil {
		return artifact, err
	}
	raw, _ := json.Marshal(artifact)
	if err := os.WriteFile(filepath.Join(staging, "artifact.json"), raw, 0600); err != nil {
		return artifact, err
	}
	if err := os.RemoveAll(source); err != nil {
		return artifact, err
	}
	if err := os.Rename(staging, root); err != nil {
		return artifact, err
	}
	return artifact, nil
}

func releaseArtifactDigest(file string) (string, error) {
	stream, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, stream); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func archiveReleaseArtifact(ctx context.Context, root, destination string) error {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return errors.New("artifact path must be a directory")
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	files := 0
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == root {
			return nil
		}
		if entry.Name() == ".git" || entry.Name() == "node_modules" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("release artifacts must contain regular files and directories")
		}
		relative, _ := filepath.Rel(root, name)
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files++
		input, err := os.Open(name)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(archive, input)
		closeErr := input.Close()
		return errors.Join(copyErr, closeErr)
	})
	err = errors.Join(err, archive.Close(), compressed.Close())
	if err == nil && files == 0 {
		return errors.New("local release artifact is empty")
	}
	return err
}

func UploadLocalReleaseArtifact(ctx context.Context, resource DeploymentResource, key []byte, directory, releaseID string, artifact LocalReleaseArtifact) error {
	file, err := os.Open(artifact.File)
	if err != nil {
		return err
	}
	defer file.Close()
	script := localArtifactUploadScript(directory, releaseID, artifact.Digest)
	return runDeploymentSSH(ctx, resource, key, "sh -ec "+sshShellQuote(script), file, io.Discard)
}

func localArtifactUploadScript(directory, releaseID, digest string) string {
	root := filepath.ToSlash(filepath.Join(directory, "releases", releaseID))
	return "set -eu\numask 077\ncommand -v tar >/dev/null\ncommand -v sha256sum >/dev/null\ncommand -v flock >/dev/null\numask 022\nmkdir -p " + sshShellQuote(filepath.ToSlash(filepath.Dir(root))) + "\numask 077\nexec 9>" + sshShellQuote(directory+"/.multica.lock") + "\nflock -n 9\nstaging=$(mktemp -d " + sshShellQuote(root+".upload.XXXXXX") + ")\ntrap 'rm -rf -- \"$staging\"' EXIT\ncat > \"$staging/archive.tar.gz\"\nprintf '%s  %s\\n' " + sshShellQuote(digest) + " \"$staging/archive.tar.gz\" | sha256sum -c - >/dev/null\n" +
		"if [ -f " + sshShellQuote(root+"/.multica-artifact") + " ] && [ \"$(cat " + sshShellQuote(root+"/.multica-artifact") + ")\" = " + sshShellQuote(digest) + " ]; then exit 0; fi\n" +
		"test ! -e " + sshShellQuote(root) + "\nmkdir \"$staging/content\"\ntar -xzf \"$staging/archive.tar.gz\" -C \"$staging/content\"\nprintf '%s' " + sshShellQuote(digest) + " > \"$staging/content/.multica-artifact\"\nmv \"$staging/content\" " + sshShellQuote(root) + "\n"
}
