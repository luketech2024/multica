# Workbench changes on the upstream baseline

Baseline 0d63bfc6556ca8a57512c818b897673b28d381c3, branch feat/workbench-development.

- Shared UI tokens and primitives implement the approved workbench theme. Preserve upstream platform tokens and theme persistence when replaying.
- Issue requirements use the existing acceptance_criteria JSONB column and integer revision compare-and-swap. Regenerate sqlc after replaying issue.sql. Requirements changes and their audit rows commit together.
- Shared issue detail renders the requirements editor. API schema rejects unsupported criteria for editing while retaining the rest of the issue response.
- Agent comments and run transcripts recognize paired wb-summary markers, preserving access to the full original output and attachments.

Validation and incomplete scope are tracked in D:\vibe-coding工作台\开发任务与验证.md. No production deployment or real-agent execution has been performed.
- Deployment targets and encrypted credentials are configured through shared project/settings views and workspace-scoped APIs. Replay migrations 615–619 and the concurrent-index cleanup registrations. MULTICA_WORKBENCH_SECRET_KEY is configured by the operator. Missing environment fields remain incomplete, and self-check/deployment execution is still pending.
- External ticket numbers use the independent issue.external_ticket_no column from migration 620. Preserve validation, non-unique duplicate hints, revision checks, atomic audits, API projections, and subtask restrictions. The system issue identifier remains immutable.
- Project teams use project_resource rows of type workbench_roles with a configuration revision. Public repository lists and counts exclude those rows, and generic resource writes cannot change them. Project and team creation share a transaction and an audit record. No project role columns or migration 621 are required.
- Workbench requirements snapshot project teams on creation. Requirement edits preserve that snapshot, and workbench subtasks inherit their parent's snapshot. Older tasks and native paths without workbench requirements still need integration with the full workflow.
- Shared project creation now uses three steps and preserves failed submissions. Project names are compared case-insensitively after trimming, with transaction-level workspace locking for create/rename. Workbench projects require a 2–40 character name and a description. Business templates, repository stack detection, and the complete scheduling/release workflow remain pending.
- Built-in business templates fork into project_resource rows of type workbench_template. Preserve the server catalog, version checks, transactional creation/audits, and the configuration-resource filters in project_resource.sql. No migration is required.
- Shared project details edit all template fields with revision checks. Manual issue creation prefills project context, criteria and separate bounds while retaining user edits. Workbench issues snapshot the server template alongside the team, and children inherit their parent's snapshots. Full custom template assets, upgrades, default role assignment and outcome gates remain pending.
