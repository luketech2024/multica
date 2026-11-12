---
name: multica-onespec-testcase
description: "Use for the testcase stage of a Multica OneSpec task. Read the current stage before acting."
user-invocable: false
allowed-tools: Bash(multica *)
---

# OneSpec testcase

Platform rules version 1.0.0. This skill is assigned to the test role.

Generate test cases from confirmed acceptance criteria and affected code. Record preconditions, steps, expected outcomes and traceability. Run the checks and submit evidence tied to the tested commit. Failed tests must not be reported as ready.

Read `multica issue onespec get <issue-id>` first. Use task-local code and documents under `onespec/changes/<issue-id>/`. Each repository keeps its own specs and the selected overview repository keeps `onespec/overview.md` with dependencies and delivery links.

For stage evidence, commit the document and compute its SHA256 digest. Send `multica issue onespec artifact <issue-id> --file <json-file>` with `expected_revision` and `artifact` containing `stage`, `summary`, `refs` with repository ID, relative path, full commit and digest. A plan additionally contains `tasks` using the execution-plan task schema. Use real values and inspect subcommand help. Never invent a successful command or evidence.

When the response is awaiting_confirmation, stop execution and direct the user to the OneSpec panel. Confirmations are human-only and tied to the displayed version. A rejected or superseded version needs corrected evidence. Refresh revisions before retrying. Continue only the currently assigned stage.
