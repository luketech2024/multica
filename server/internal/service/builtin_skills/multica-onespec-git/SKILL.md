---
name: multica-onespec-git
description: "Use for the git stage of a Multica OneSpec task. Read the current stage before acting."
user-invocable: false
allowed-tools: Bash(multica *)
---

# OneSpec git

Platform rules version 1.0.0. This skill is assigned to the git role.

Use release as business base. Prepare release_{task_id}, hotfix_{YYYYMMDD}{task_id}, docs_{description}, or main_{YYYYMMDD} as appropriate. Record base commit before work. Use a separate worktree and verify existing branch ownership before reuse. Fork updates enter main through main_{YYYYMMDD}, then integrate into release. Record actual conflicts and resolutions. Merge deployed changes back to release after successful deployment. Without an explicit policy request human authorization before merge, push or tags. Preserve unrelated work on retry. Record operation key, repository_id, kind, branch, worktree, base_commit, status, summary and actual published/merged commits through the Git API.

Read `multica issue onespec get <issue-id>` first. Use task-local code and documents under `onespec/changes/<issue-id>/`. Each repository keeps its own specs and the selected overview repository keeps `onespec/overview.md` with dependencies and delivery links.

For stage evidence, commit the document and compute its SHA256 digest. Send `multica issue onespec artifact <issue-id> --file <json-file>` with `expected_revision` and `artifact` containing `stage`, `summary`, `refs` with repository ID, relative path, full commit and digest. A plan additionally contains `tasks` using the execution-plan task schema. Use real values and inspect subcommand help. Never invent a successful command or evidence.

When the response is awaiting_confirmation, stop execution and direct the user to the OneSpec panel. Confirmations are human-only and tied to the displayed version. A rejected or superseded version needs corrected evidence. Refresh revisions before retrying. Continue only the currently assigned stage.

For branch preparation, use the bundled `scripts/prepare.py` with an available Python interpreter. Pass `--repo`, the immutable `--task`, the validated `--branch`, fixed `--base` commit and isolated `--directory`. It records ownership in Git metadata and refuses reuse with a different task or base. It performs local preparation only. Read the returned commit and worktree path, then record them through the workflow API. In merge stage, prepare a reviewable merge proposal. Execute the approved proposal in merging stage and record its actual merge commit.
