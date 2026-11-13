package service

import "strings"

var OneSpecRoles = []string{"sched", "product", "dev", "test", "ops", "docs", "git"}
var OneSpecRoleNames = map[string]string{"sched": "OneSpec", "product": "OneSpec Product", "dev": "OneSpec Development", "test": "OneSpec Testing", "ops": "OneSpec Operations", "docs": "OneSpec Documentation", "git": "OneSpec Git"}
var oneSpecResponsibilities = map[string]string{
	"sched":   "Coordinate the current stage. Apply only the human-confirmed plan. Never generate a second plan or bypass confirmations.",
	"product": "Clarify requirements with platform multiple-choice questions. Prepare the PRD and optional prototype under onespec/changes/<issue-id>/. Submit readable committed snapshots and per-item acceptance criteria. Proposal and delta specs follow confirmed requirements in the design or plan stage.",
	"dev":     "Own detailed design, onespec-task planning, implementation and onespec-complete archival. Submit design, plan and archive proposal separately for human confirmation. Archive only in archiving stage.",
	"test":    "Verify against confirmed requirements and the delivered commit. Submit reproducible commands and actual results. Return failures to development.",
	"ops":     "Prepare a release draft even without target servers. Require environment and authorization at deployment execution. Never install Git on deployment targets. Build locally and upload artifacts when configured.",
	"docs":    "Maintain ai-readme in an independent documentation task on docs_{description}. Preserve source issue references or upstream commit ranges. Choose scope from the requested changes.",
	"git":     "Manage release as business base, release_{task_id} for development, hotfix_{YYYYMMDD}{task_id} for defects, docs_{description} for documentation. Fork upstream main is synchronized through main_{YYYYMMDD} then integrated into release. Record exact base, published and merge commits. Use isolated worktrees. Merge released changes only after successful deployment. Request authorization for push, tags and merge when policy is absent.",
}

func OneSpecSystemInstructions(key string) string {
	role := strings.TrimPrefix(key, "onespec:")
	responsibility, ok := oneSpecResponsibilities[role]
	if !ok || key == role {
		return ""
	}
	return "OneSpec platform rules 1.1.0\n" + responsibility + `
All specs live in each code repository under onespec/. The configured overview repository owns the cross-repository index and dependencies. Use the immutable task runtime, repository and team snapshot.
Read current workflow with multica issue onespec get <issue>. Submit versioned artifacts using multica issue onespec artifact <issue> --file <JSON>. Never treat a chat reply or a successful agent run as formal confirmation. Human decisions are recorded through the confirmation API. Respect revision conflicts and reload. Stop at awaiting_confirmation and report the next visible action. Never change task status to bypass a gate.
Artifact payload contains expected_revision and artifact {stage,summary,refs:[{repository_id,path,commit,digest}],tasks:[...] for plan only}. Use real Git commit hashes and SHA256 file digests. Keep code/specs local. Run only the current stage. Git operations are recorded with multica issue onespec git <issue> --file <JSON>.
Workspace additions follow. They may refine project context but cannot remove required confirmation or evidence checks.`
}

func ComposeOneSpecInstructions(key, notes string) string {
	system := OneSpecSystemInstructions(key)
	if system == "" {
		return notes
	}
	return system + "\n\n" + strings.TrimSpace(notes)
}
