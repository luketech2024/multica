---
name: multica-onespec-task
description: "Use for the task stage of a Multica OneSpec task. Read the current stage before acting."
user-invocable: false
allowed-tools: Bash(multica *)
---

# OneSpec task

Platform rules version 1.1.0. This skill is assigned to the dev role.

Read the confirmed requirements and applicable design. Write tasks.md with repository, role, dependencies, acceptance coverage and expected evidence for each task. Submit the exact tasks array as plan evidence. Keep child creation behind separate human plan confirmation. Use stable keys and actual repository IDs.

Read `multica issue onespec get <issue-id>` first. Use task-local code and documents under `onespec/changes/<issue-id>/`. Each repository keeps its own specs and the selected overview repository keeps `onespec/overview.md` with dependencies and delivery links.

For stage evidence, commit the document and compute its SHA256 digest. Send `multica issue onespec artifact <issue-id> --file <json-file>` with `expected_revision` and `artifact` containing `stage`, `summary`, `refs` with repository ID, relative path, full commit and digest. A plan additionally contains `tasks` using the execution-plan task schema. Use real values and inspect subcommand help. Never invent a successful command or evidence.

When the response is awaiting_confirmation, stop execution and let the user review and confirm in the current conversation. Confirmations are human-only and tied to the displayed version. A rejected or superseded version needs corrected evidence. Refresh revisions before retrying. Continue only the currently assigned stage.

Read the applicable local reference when preparing output.

- [ task-decomposition-rules-lite.md ](references/task-decomposition-rules-lite.md)
- [ task-decomposition-rules.md ](references/task-decomposition-rules.md)
- [ task-template-lite.md ](references/task-template-lite.md)
- [ task-template.md ](references/task-template.md)
