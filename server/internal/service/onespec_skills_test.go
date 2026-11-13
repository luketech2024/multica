package service

import (
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestOneSpecSkillVersionAndStageOwnership(t *testing.T) {
	for _, role := range OneSpecRoles {
		for _, skill := range loadBuiltinSkills("onespec:" + role) {
			if !strings.HasPrefix(skill.Name, "multica-onespec-") {
				continue
			}
			if !strings.Contains(skill.Content, "Platform rules version "+workbench.OneSpecVersion) {
				t.Fatalf("%s has a stale contract", skill.Name)
			}
			if skill.Name == "multica-onespec-change" && role != "dev" {
				t.Fatal("proposal assigned before requirement confirmation")
			}
			if skill.Name == "multica-onespec-generate-prd" && !strings.Contains(skill.Content, "--repository-dir") {
				t.Fatal("PRD skill omits verified publication")
			}
		}
		if !strings.Contains(OneSpecSystemInstructions("onespec:"+role), workbench.OneSpecVersion) {
			t.Fatal("role version differs from workflow")
		}
	}
}
