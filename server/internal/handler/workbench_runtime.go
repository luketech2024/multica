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
	discussion := ""
	if !issue.ParentIssueID.Valid && criteria.Gate.Spec == nil && (criteria.Phase == "clarifying" || criteria.Phase == "draft") {
		discussion = `

## Current turn: clarify the request in conversation
This takes priority over role defaults about immediately preparing acceptance criteria, dispatching work, or reading repositories. The assigned agent owns this conversation, regardless of its role name.
Read the task and current discussion with multica issue comment list <issue> --full --output json. Then make your first substantive reply a short statement of what is understood and ask about the missing goal, scope or success condition. A short deployment request plus a branch name does not by itself establish a complete deployment scope or acceptance criteria.
When material information is missing, submit 1-3 focused choices with multica issue questions <issue> --file <JSON> and end the turn. Payload {key:"stable-question-key",questions:[{id:"scope",header:"Scope",question:"...",isOther:true,options:[{label:"...",description:"...",recommended:true}]}]}. Use the user's language. Reuse known answers, allow free text, and offer to defer execution details. Ask for a repository or application identity if the target is unclear; server credentials can wait until execution. If the question tool fails, ask the same questions in the conversation and stop.
Until those questions are answered, defer repository exploration, PRD generation, acceptance submission, planning and deployment. Do not investigate platform internals, scan installed binaries or search unrelated local directories to prepare a question. If the current discussion already supplies the material requirements, summarize them and prepare the review without forcing another question round.
`
	}
	return discussion + `

## Project workflow contract
This project uses structured requirements, plans and delivery evidence. Apply these project-specific steps when older role defaults ask for mandatory clarification rounds or comment-only handoffs.
The issue already exists. Do not create a duplicate parent or unplanned children.
At EVERY stage, including planned child execution, testing and deployment, a decision needed from the user must be a structured question. Use multica issue questions <current-issue-id> --file <JSON> with {key:"stable-decision-key",questions:[{id:"decision",header:"Decision",question:"...",isOther:true,options:[{label:"...",description:"...",recommended:true}]}]}. For a child, use the child's issue ID; its questions are also displayed in the parent conversation. For example, a choice between keeping Podman and installing Docker belongs in this interface, not only in the final reply. Only ask for a material decision not already settled by the user or the confirmed scope. First read existing questions and answers with multica issue questions <current-issue-id>; reuse an unanswered question instead of duplicating it. After the create command succeeds, end the turn and wait. A platform question survives the end of the turn, and the user's answer resumes the assigned agent on the same issue. Do not mark the issue blocked or delivered merely because you are waiting for an answer. Do not execute the dependent action or choose on the user's behalf. If question creation fails, report that failure and the decision explicitly; never claim that an interactive question was created when it was not.
For a parent without confirmed requirements: the title names a discussion, it is not a complete request. Use the description and user messages as the source of requirements. First understand the goal and ask only for missing decisions that materially affect scope or acceptance. Do not invent answers, require a fixed number of rounds, or demand server credentials that can be supplied before execution. Keep the parent in todo and keep the current discussion agent assigned until human confirmation. The confirmer is independent of the assigned agent. Do not assign the task to a member to request confirmation.
Save drafts with multica issue workflow requirements <issue> --file <UTF-8 JSON>. Payload {expected_revision:<fresh issue revision>,acceptance_criteria:{v:1,phase:"clarifying" or "draft",covered_discussion_revision:<discussion_revision actually read>,items:[{id:"",text:"...",origin:"agent",covers:[],checked:null,note:null}],bounds:[]}}. Preserve existing item IDs. Use bounds for scope exclusions and explicitly deferred execution prerequisites. Saving a draft does not request confirmation.
Once material questions are resolved, submit the current draft with phase:"ready" and covered_discussion_revision from the messages you actually processed. Before updating coverage, read multica issue comment list <issue> --full --output json. Summary, roots-only and folded reads do not record full discussion coverage. Use acceptance_criteria.discussion_revision from the refreshed issue; an omitted value means 0. New or edited/deleted messages invalidate prior coverage. Never copy a newer revision without reading the corresponding discussion. For an answer that does not change requirements, preserve all items and boundaries, answer in chat and resubmit phase:"ready" with current coverage. Content changes produce a new draft revision and reset affected checks. Never check items or confirm for the user. Plain "OK" or "continue" is conversation, not formal approval. For OneSpec, submit the structured draft before the requirements artifact; then use the existing OneSpec stages after human confirmation. Waiting for a user answer is normal, not blocked. Use multica issue questions <issue> --file <JSON> for runtime-independent choices. Submit 1-3 material questions with a stable key, questions [{id,header,question,isOther:true,options:[{label,description,recommended}]}]. End the turn after asynchronous questions and process answers from the discussion on the next run. In OneSpec requirements, publish a committed PRD snapshot with multica issue onespec artifact <issue> --file <JSON> --repository-dir <checkout>. Include artifact.document.ref and optional document.prototype.ref. Runtime verification must succeed before confirmation. Read the platform multica-onespec skills for this workflow; personal workflow skills must not replace the platform lifecycle.
Only for a project WITHOUT an initialized OneSpec workflow, after human confirmation the assigned project scheduler saves one plan with multica issue workflow plan <parent> --file <JSON>. Use expected_revision, version, a stable idempotency_key, tasks [{key,title,description,role,repo_id,covers,depends_on,serial_group,expected}]. Include only roles required by the confirmed scope, actual project repository IDs and full A-xx coverage. Enabled roles are available options, not mandatory subtasks. For deployment-only requests, use the bound ops role for deployment analysis and execution; add other roles only when the confirmed scope requires their work. Before drafting the plan, inspect the supplied team snapshot for each selected role. If a required role has no agent, report the exact role and ask the user to configure it in project team settings, then stop. Do not create unassigned automated subtasks. Save the whole plan in one call. The system creates and dispatches children. Do not manually create or dispatch additional children. Refresh on revision conflict and inspect an existing plan before retrying. On a validation error, fix the named field once using the documented schema. On a role or permission error, report the missing configuration and end the turn. Do not scan installed binaries or unrelated directories to infer platform fields.
For a planned child: perform the assigned work, publish real verification evidence, then use multica issue workflow delivery <parent> --file <JSON>. Payload {expected_revision:<fresh parent revision>,evidence:{issue_id:<child>,issue_revision:<fresh child revision>,commit:<full source hash>,passed:<actual result>,summary:<conclusion>,reports:[{name:<expected report name>,url:<real HTTP(S) report URL>}],changes:{files,additions,deletions},preview_url:<verified HTTP(S) preview URL>}}. The endpoint atomically records evidence and completes the child. Keep the child in_progress until this succeeds. Human acceptance applies to the parent after all runs settle. Never fabricate a report, preview or merge. If evidence cannot be provided, state the concrete blocker. A transcript comment containing verification results can be used as a report via its actual issue comment URL.
Use multica issue workflow --help and its subcommand help for command details. Always refresh revisions before writes. After a delivery submission, do not update the child again unless correcting the evidence with its new revision.
Current structured requirements: ` + string(snapshot) + "\nCurrent planned handoff: " + string(handoff)
}
