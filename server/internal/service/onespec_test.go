package service

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOneSpecGitPreparation(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("python3 is required for the bundled Git helper verification")
	}
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-b", "release")
	git("config", "user.name", "OneSpec test")
	git("config", "user.email", "onespec@example.test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "README.md")
	git("commit", "-m", "fixture")
	base := git("rev-parse", "HEAD")
	script, err := filepath.Abs("builtin_skills/multica-onespec-git/scripts/prepare.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, branch := range []string{"release_task1", "hotfix_20261111task2", "docs_upstream-update", "main_20261111"} {
		destination := filepath.Join(t.TempDir(), "work")
		args := []string{script, "--repo", repo, "--task", branch, "--branch", branch, "--base", base, "--directory", destination}
		for n := 0; n < 2; n++ {
			output, e := exec.Command(python, args...).CombinedOutput()
			if e != nil {
				t.Fatalf("prepare %s: %v %s", branch, e, output)
			}
			var record map[string]string
			if json.Unmarshal(output, &record) != nil || record["base_commit"] != base {
				t.Fatalf("invalid preparation record: %s", output)
			}
		}
		args[4] = "other-task"
		if output, e := exec.Command(python, args...).CombinedOutput(); e == nil {
			t.Fatalf("foreign task reused worktree: %s", output)
		}
	}
	if got := git("branch", "--show-current"); got != "release" {
		t.Fatalf("source checkout changed to %s", got)
	}
}

func TestOneSpecRoleSkillDistribution(t *testing.T) {
	dev := loadBuiltinSkills("onespec:dev")
	names := map[string]bool{}
	for _, skill := range dev {
		names[skill.Name] = true
	}
	if !names["multica-onespec-task"] || !names["multica-onespec-complete"] || names["multica-onespec-release"] {
		t.Fatal("development received the wrong workflow skills")
	}
	for _, skill := range loadBuiltinSkills("") {
		if strings.HasPrefix(skill.Name, "multica-onespec-") {
			t.Fatal("ordinary agents received role-specific workflow skills")
		}
	}
	for _, role := range OneSpecRoles {
		if OneSpecSystemInstructions("onespec:"+role) == "" {
			t.Fatalf("missing platform rules for %s", role)
		}
	}
}

func TestOneSpecSkillNamespaceCollision(t *testing.T) {
	for _, name := range []string{"multica-onespec-git", "Multica OneSpec Git", " multica_onespec_git "} {
		if ValidateOneSpecSkillNames([]AgentSkillData{{Name: name}}) == nil {
			t.Fatalf("accepted platform collision %q", name)
		}
	}
	if err := ValidateOneSpecSkillNames([]AgentSkillData{{Name: "project-git"}}); err != nil {
		t.Fatal(err)
	}
}
