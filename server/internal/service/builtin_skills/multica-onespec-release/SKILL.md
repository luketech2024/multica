---
name: multica-onespec-release
description: "Use for the release stage of a Multica OneSpec task. Read the current stage before acting."
user-invocable: false
allowed-tools: Bash(multica *)
---

# OneSpec release

Platform rules version 1.1.0. This skill is assigned to the ops role.

Prepare a release draft from the selected applications and immutable artifact versions. An absent server is a draft configuration gap. Execute only after the release execution API validates environment, credentials and human authorization. Build locally and upload artifacts to targets that have no Git. Record health checks, actual published commit and rollback result. Submit deployment evidence and hand off release integration to Git management.

Read `multica issue onespec get <issue-id>` first. Use task-local code and documents under `onespec/changes/<issue-id>/`. Each repository keeps its own specs and the selected overview repository keeps `onespec/overview.md` with dependencies and delivery links.

For stage evidence, commit the document and compute its SHA256 digest. Send `multica issue onespec artifact <issue-id> --file <json-file>` with `expected_revision` and `artifact` containing `stage`, `summary`, `refs` with repository ID, relative path, full commit and digest. A plan additionally contains `tasks` using the execution-plan task schema. Use real values and inspect subcommand help. Never invent a successful command or evidence.

When the response is awaiting_confirmation, stop execution and let the user review and confirm in the current conversation. Confirmations are human-only and tied to the displayed version. A rejected or superseded version needs corrected evidence. Refresh revisions before retrying. Continue only the currently assigned stage.

Create the linked application release draft using `multica issue onespec release-draft <issue-id> --file <json-file>` with `expected_revision` and `release` containing `env` (staging or prod) and optional `delivery` content. Empty drafts are allowed, so missing application or server configuration can be completed in the release center. Submit release_preparation evidence only after the draft exists. Deployment evidence is accepted only after this linked release is live.
