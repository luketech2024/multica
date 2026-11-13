---
name: multica-onespec-generate-prd
description: "Use for the generate-prd stage of a Multica OneSpec task. Read the current stage before acting."
user-invocable: false
allowed-tools: Bash(multica *)
---

# OneSpec generate-prd

Platform rules version 1.1.0. This skill is assigned to the product role.

Start with the task and its conversation. When the goal, scope or success condition is incomplete, make the first substantive reply a short understanding of the request and 1-3 focused choices using `multica issue questions <issue-id> --file <json>`, then end the turn. Follow the question schema below. Reuse supplied answers and allow free text. Defer repository exploration and PRD generation until material questions are answered. If questions cannot be submitted, ask in the conversation and stop instead of investigating platform internals. A deployment title and branch alone do not establish acceptance criteria. Server credentials may be supplied before execution.

Once the discussion supplies the material requirements, read the relevant repository context and write a concise PRD with explicit acceptance criteria, scope and resolved decisions. Save structured acceptance items using multica issue workflow requirements. Submit the committed PRD as requirements evidence. Wait for the creator or project lead to confirm. Do not force an extra question round when the user already supplied sufficient information.

Before preparing stage evidence, read `multica issue onespec get <issue-id>`. Use task-local code and documents under `onespec/changes/<issue-id>/`. Each repository keeps its own specs and the selected overview repository keeps `onespec/overview.md` with dependencies and delivery links.

For stage evidence, commit the document and compute its SHA256 digest. Send `multica issue onespec artifact <issue-id> --file <json-file>` with `expected_revision` and `artifact` containing `stage`, `summary`, `refs` with repository ID, relative path, full commit and digest. A plan additionally contains `tasks` using the execution-plan task schema. Use real values and inspect subcommand help. Never invent a successful command or evidence.

When the response is awaiting_confirmation, stop execution and let the user review and confirm in the current conversation. Confirmations are human-only and tied to the displayed version. A rejected or superseded version needs corrected evidence. Refresh revisions before retrying. Continue only the currently assigned stage.

Read the applicable local reference when preparing output.

- [ grading-system.md ](references/grading-system.md)
- [ prd-ears-style.md ](references/prd-ears-style.md)
- [ requirement-analysis.md ](references/requirement-analysis.md)

## Discussion and review contract

Read the full discussion with `multica issue comment list <issue-id> --full --output json` before saving requirements. Summary, roots-only and folded reads do not record full discussion coverage. Use `acceptance_criteria.discussion_revision` from the current issue, treating an omitted value as 0. If a revision conflict occurs, read the full discussion again and refresh the issue before retrying.

Use the platform workflow and this skill even when personal OneSpec skills are visible. Ask only unresolved material questions through `multica issue questions <issue-id> --file <json>`. Use a stable request key and 1-3 questions with id, header, question, isOther=true, and options with label, description and optional recommended=true. A recommendation requires an explicit user answer. Allow deployment resources to be supplied before execution when they do not change scope. End the turn after asynchronous questions. On the next run, read the entire current discussion, including corrections, before updating covered_discussion_revision.

Organize the PRD into five readable sections: purpose and users, scope and exclusions, user interactions and behavior, constraints and deferred information, and verifiable acceptance. Link each structured acceptance item to its matching Markdown section using section and category (purpose, scope, behavior, constraints, acceptance). Ask whether a prototype is useful. If requested, generate a self-contained HTML prototype with labeled sample data under the same change directory, and include it in the same submitted requirement version. Keep a draft while it is incomplete or failed.

Commit PRD and optional prototype before publishing. The artifact request includes expected_revision, artifact.stage=requirements, summary, and document.ref {repository_id,path,commit}. For an optional prototype add document.prototype.ref with the same fields. Run `multica issue onespec artifact <issue-id> --file <json> --repository-dir <checkout>`. The CLI reads committed bytes and hashes; the daemon independently verifies the files before human confirmation. If verification fails, correct the file/ref and resubmit a new version. Never set verified yourself. Existing refs are retained as evidence, while the readable snapshot is what the user reviews.

Preserve unaffected confirmation items when answering explanations. Changed content or prototype behavior must update the relevant acceptance items and require reconfirmation. Use the exact confirmed document reference in later stages. Feature tasks proceed to design; change/bugfix tasks proceed to a plan that includes the proposal and delta specs. Release and documentation tasks follow their existing stage sequence. Never bypass stage confirmation or create a competing scheduler plan.

Record document.prototype_status as preparing or failed when an explicitly requested prototype is incomplete, and ready with the prototype file when available. Such incomplete versions remain drafts and cannot be confirmed.
