package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) workbenchSourceTaskID(r *http.Request, issue db.Issue, actorType, actorID string) pgtype.UUID {
	task, ok := h.taskFromRequestHeader(r)
	if actorType != "agent" || !ok || task.IssueID != issue.ID || uuidToString(task.AgentID) != actorID || task.Status != "running" {
		return pgtype.UUID{}
	}
	return task.ID
}

func workbenchRuntimeInstructions(issue db.Issue) string {
	criteria, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
	handoff := workbench.HandoffJSON(issue.Properties)
	if err != nil || criteria.Team == nil && len(handoff) == 0 {
		return ""
	}
	snapshot, _ := json.Marshal(criteria)
	return `

## Project workflow contract
This project uses structured requirements, plans and delivery evidence. Apply these project-specific steps when older role defaults ask for mandatory clarification rounds or comment-only handoffs.
The issue already exists. Do not create a duplicate parent or unplanned children.
For a parent without confirmed requirements: derive a short set of independently verifiable acceptance items from the user's explicit request. Ask only about missing decisions that materially affect scope, safety or acceptance. Do not require a fixed number of questions. Keep the parent in todo while preparing requirements. Save them using multica issue workflow requirements <issue> --file <UTF-8 JSON>. Use expected_revision from a fresh issue read and acceptance_criteria {v:1,items:[{id:"",text:"...",origin:"agent",covers:[],checked:null,note:null}],bounds:[]}. Preserve existing item IDs when editing. New IDs are assigned by the server. Direct the human once to the Requirements confirmation button. Do not ask for repeated chat approvals or confirm on their behalf.
After human confirmation, the assigned project scheduler saves one plan with multica issue workflow plan <parent> --file <JSON>. Use expected_revision, version, a stable idempotency_key, tasks [{key,title,description,role,repo_id,covers,depends_on,serial_group,expected}]. Include enabled dev/test/sec/ops roles, actual project repository IDs and full A-xx coverage. Save the whole plan in one call. The system creates and dispatches children. Do not manually create or dispatch additional children. Refresh on revision conflict and inspect an existing plan before retrying.
For a planned child: perform the assigned work, publish real verification evidence, then use multica issue workflow delivery <parent> --file <JSON>. Payload {expected_revision:<fresh parent revision>,evidence:{issue_id:<child>,issue_revision:<fresh child revision>,commit:<full source hash>,passed:<actual result>,summary:<conclusion>,reports:[{name:<expected report name>,url:<real HTTP(S) report URL>}],changes:{files,additions,deletions},preview_url:<verified HTTP(S) preview URL>}}. The endpoint atomically records evidence and completes the child. Keep the child in_progress until this succeeds. Human acceptance applies to the parent after all runs settle. Never fabricate a report, preview or merge. If evidence cannot be provided, state the concrete blocker. A transcript comment containing verification results can be used as a report via its actual issue comment URL.
Use multica issue workflow --help and its subcommand help for command details. Always refresh revisions before writes. After a delivery submission, do not update the child again unless correcting the evidence with its new revision.
Current structured requirements: ` + string(snapshot) + "\nCurrent planned handoff: " + string(handoff)
}
