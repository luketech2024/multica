---
name: multica-onespec-documentation
description: "Use for the documentation stage of a Multica OneSpec task. Read the current stage before acting."
user-invocable: false
allowed-tools: Bash(multica *)
---

# OneSpec documentation

Platform rules version 1.0.0. This skill is assigned to the docs role.

Use an independent documentation task and docs_{description} branch. Read the requested code changes, source tasks or upstream commit range. Update ai-readme and verify links and code references. Record source revisions even when no originating task exists. Submit documentation evidence and await verification and merge approval.

Read `multica issue onespec get <issue-id>` first. Use task-local code and documents under `onespec/changes/<issue-id>/`. Each repository keeps its own specs and the selected overview repository keeps `onespec/overview.md` with dependencies and delivery links.

For stage evidence, commit the document and compute its SHA256 digest. Send `multica issue onespec artifact <issue-id> --file <json-file>` with `expected_revision` and `artifact` containing `stage`, `summary`, `refs` with repository ID, relative path, full commit and digest. A plan additionally contains `tasks` using the execution-plan task schema. Use real values and inspect subcommand help. Never invent a successful command or evidence.

When the response is awaiting_confirmation, stop execution and direct the user to the OneSpec panel. Confirmations are human-only and tied to the displayed version. A rejected or superseded version needs corrected evidence. Refresh revisions before retrying. Continue only the currently assigned stage.

Use the ai-readme references for code analysis and documentation structure. The OneSpec stage and confirmation contract above governs execution.
- [ai-readme-cocreate-doc](references/ai-readme-cocreate-doc/SKILL.md)
- [ai-readme-code-review](references/ai-readme-code-review/SKILL.md)
- [ai-readme-workspace](references/ai-readme-workspace/SKILL.md)
- [ai-readme](references/ai-readme/SKILL.md)
