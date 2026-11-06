# Workbench changes on the upstream baseline

Baseline 0d63bfc6556ca8a57512c818b897673b28d381c3, branch feat/workbench-development.

- Shared UI tokens and primitives implement the approved workbench theme. Preserve upstream platform tokens and theme persistence when replaying.
- Issue requirements use the existing acceptance_criteria JSONB column and integer revision compare-and-swap. Regenerate sqlc after replaying issue.sql. Requirements changes and their audit rows commit together.
- Shared issue detail renders the requirements editor. API schema rejects unsupported criteria for editing while retaining the rest of the issue response.
- Agent comments and run transcripts recognize paired wb-summary markers, preserving access to the full original output and attachments.

Validation and incomplete scope are tracked in D:\vibe-coding工作台\开发任务与验证.md. No production deployment or real-agent execution has been performed.
