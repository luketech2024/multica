---
name: multica-onespec-change
description: "Prepare proposal and delta specs after requirements confirmation, during the design or plan stage of a Multica OneSpec task."
user-invocable: false
allowed-tools: Bash(multica *)
---

# OneSpec change

Platform rules version 1.1.0. This skill is assigned to the dev role.

After PRD confirmation, development writes onespec/changes/<issue-id>/proposal.md and specs/*.md using the confirmed requirements, affected behavior and boundaries. Update the configured overview repository index for multi-repository tasks. Attach these files to the design artifact for feature tasks, or the plan artifact for change/bugfix tasks.

Read `multica issue onespec get <issue-id>` first. Use task-local code and documents under `onespec/changes/<issue-id>/`. Each repository keeps its own specs and the selected overview repository keeps `onespec/overview.md` with dependencies and delivery links.

For stage evidence, commit the document and compute its SHA256 digest. Send `multica issue onespec artifact <issue-id> --file <json-file>` with `expected_revision` and `artifact` containing `stage`, `summary`, `refs` with repository ID, relative path, full commit and digest. A plan additionally contains `tasks` using the execution-plan task schema. Use real values and inspect subcommand help. Never invent a successful command or evidence.

When the response is awaiting_confirmation, stop execution and let the user review and confirm in the current conversation. Confirmations are human-only and tied to the displayed version. A rejected or superseded version needs corrected evidence. Refresh revisions before retrying. Continue only the currently assigned stage.

Read the applicable local reference when preparing output.

- [ change-ears-style.md ](references/change-ears-style.md)
- [ proposal-template.md ](references/proposal-template.md)
- [ spec-template.md ](references/spec-template.md)
