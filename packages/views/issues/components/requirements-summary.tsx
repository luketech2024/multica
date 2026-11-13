"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { useUpdateIssue } from "@multica/core/issues/mutations";
import { projectDetailOptions } from "@multica/core/projects";
import { oneSpecFlowOptions, oneSpecStages, useContinueRequirements, requirementsHandoff, requirementsReady, requirementsReviewReady } from "@multica/core/workbench/configuration";
import { issueTasksOptions } from "@multica/core/issues/queries";
import { useCommentDraftStore } from "@multica/core/issues/stores";
import type { Issue } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { ReadonlyContent } from "../../editor/readonly-content";
import { HtmlBlockPreview } from "../../editor/html-block-preview";
import { userInputOptions } from "@multica/core/issues/user-input";
import { requirementSection } from "@multica/core/workbench/configuration";

export function RequirementsSummary({ issue, onDiscuss }: { issue: Issue; onDiscuss?: () => void }) {
  const { t } = useT("issues");
  const { t: projectText } = useT("projects");
  const wsId = useWorkspaceId();
  const user = useAuthStore((state) => state.user);
  const project = useQuery(projectDetailOptions(wsId, issue.project_id ?? ""));
  const workflow = useQuery(oneSpecFlowOptions(wsId, issue.id));
  const questions = useQuery(userInputOptions(wsId, issue.id));
  const update = useUpdateIssue();
  const action = useContinueRequirements(wsId, issue.id);
  const runs = useQuery(issueTasksOptions(issue.id));
  const draft = useCommentDraftStore((state) => state.drafts[`new:${issue.id}`]);
  const [retainedDraft, setRetainedDraft] = useState<typeof draft>();
  const [error, setError] = useState<string>();
  const criteria = issue.acceptance_criteria;
  const flow = workflow.data?.flow;
  if (!criteria?.items.length || (criteria.gate.spec && !requirementsHandoff(flow ?? null))) return null;
  const artifact = flow?.artifacts.findLast((entry) => entry.stage === "requirements");
  const document = artifact?.document;
  const previous = flow?.artifacts.filter((entry) => entry.stage === "requirements").at(-2)?.document;
  const canConfirm = flow ? workflow.data?.can_confirm : !criteria.phase ||
    (issue.creator_type === "member" && issue.creator_id === user?.id) ||
    (project.data?.lead_type === "member" && project.data.lead_id === user?.id);
  const ready = requirementsReady(criteria);
  const covered = (criteria.discussion_revision ?? 0) === (criteria.covered_discussion_revision ?? 0);
  const submitted = workflow.isSuccess && questions.isSuccess && !questions.data.some((entry) => entry.status === "pending") && requirementsReviewReady(criteria, flow);
  const busy = update.isPending || action.isPending;
  const pendingChange = !!draft?.requirementReference && !!draft.content.trim() && retainedDraft !== draft;
  const stageNames: Record<string, string> = {
    design: projectText(($) => $.onespec.design), plan: projectText(($) => $.onespec.plan),
    branch: projectText(($) => $.onespec.branch), release_preparation: projectText(($) => $.onespec.release_preparation),
  };
  const nextLabel = stageNames[oneSpecStages(flow?.kind ?? "")[1] ?? ""] ?? "";
  const categories: Record<string, string> = {
    purpose: t(($) => $.discussion.purpose), scope: t(($) => $.discussion.scope),
    behavior: t(($) => $.discussion.behavior), constraints: t(($) => $.discussion.constraints),
    acceptance: t(($) => $.discussion.acceptance),
  };
  const saveCheck = async (id: string, checked: boolean) => {
    setError(undefined);
    try {
      await update.mutateAsync({ id: issue.id, expected_revision: issue.revision,
        acceptance_criteria: { ...criteria, items: criteria.items.map((item) => item.id === id
          ? { ...item, checked: checked ? { at: "", by: "" } : null } : item) } });
    } catch (cause) { setError(cause instanceof Error ? cause.message : t(($) => $.requirements.save_failed)); }
  };
  const confirm = async () => {
    if (pendingChange || issue.workbench_pause || busy) return;
    setError(undefined);
    try {
      if (flow && artifact) {
        await action.mutateAsync(artifact.version);
      } else {
        await update.mutateAsync({ id: issue.id, expected_revision: issue.revision, confirm_requirements: true });
      }
    } catch (cause) { setError(cause instanceof Error ? cause.message : t(($) => $.requirements.save_failed)); }
  };
  const draftNotice = pendingChange && <div role="status" className="space-y-2 text-caption">
    <p>{t(($) => $.discussion.unsent_change)}</p>
    {onDiscuss && <Button size="xs" variant="ghost" onClick={onDiscuss}>{t(($) => $.discussion.edit_draft)}</Button>}
    <Button size="xs" variant="ghost" onClick={() => setRetainedDraft(draft)}>{t(($) => $.discussion.keep_draft)}</Button>
  </div>;
  if (criteria.gate.spec) {
    const confirmedAt = flow?.decisions.findLast((decision) => decision.stage === "requirements" && decision.approved)?.created_at;
    const nextRole = flow?.phase === "branch" ? "git" : flow?.phase === "release_preparation" ? "ops" : "dev";
    const nextAgent = flow?.team.config.roles[nextRole].agent_id;
    const stageRuns = flow?.phase === "requirements" ? [] : (runs.data ?? []).filter((run) => run.agent_id === nextAgent
      && (!confirmedAt || Date.parse(run.created_at) >= Date.parse(confirmedAt)));
    const active = stageRuns.find((run) => ["queued", "deferred", "dispatched", "running", "waiting_local_directory"].includes(run.status));
    let handoffMessage = t(($) => stageRuns.length ? $.discussion.confirmed_incomplete : $.discussion.confirmed_pending, { stage: nextLabel });
    if (active) handoffMessage = t(($) => active.status === "running" ? $.discussion.confirmed_running : $.discussion.confirmed_queued, { stage: nextLabel });
    if (busy) handoffMessage = t(($) => $.discussion.confirmed_starting, { stage: nextLabel });
    if (issue.workbench_pause) handoffMessage = t(($) => $.discussion.confirmed_paused);
    return <section data-testid="requirements-handoff" className="space-y-2 py-3 text-body">
      <p role="status">{handoffMessage}</p>
      {!active && !issue.workbench_pause && canConfirm && <>
        {draftNotice}
        <Button size="sm" disabled={busy || pendingChange || !runs.isSuccess} onClick={() => void confirm()}>{t(($) => $.discussion.start_stage, { stage: nextLabel })}</Button>
      </>}
      {error && <p role="alert" className="text-caption text-destructive">{error}</p>}
    </section>;
  }
  const verification = document?.error || document?.prototype?.error ||
    (document?.prototype_status === "failed" ? t(($) => $.discussion.prototype_failed) :
      document?.prototype_status === "preparing" ? t(($) => $.discussion.prototype_preparing) :
        document && (!document.verified || (document.prototype && !document.prototype.verified)) ? t(($) => $.discussion.verifying) :
          !covered ? t(($) => $.discussion.processing) : null);
  return <section data-testid="requirements-summary" className="space-y-3 rounded-xl bg-muted/40 p-4">
    <h3 className="text-label font-medium">{t(($) => ready && submitted ? $.discussion.ready : $.discussion.draft)} · {t(($) => $.requirements.version, { version: criteria.draft_revision ?? criteria.version })}</h3>
    {artifact?.summary && <p className="whitespace-pre-wrap text-body">{artifact.summary}</p>}
    {previous && document && previous.content !== document.content && <p className="text-caption text-muted-foreground">{t(($) => $.discussion.version_changed)}</p>}
    <ul className="space-y-3">{criteria.items.map((item, index) => <li key={item.id}>
      {item.category && item.category !== criteria.items[index - 1]?.category && categories[item.category] && <p className="mb-1 text-caption text-muted-foreground">{categories[item.category]}</p>}
      <div className="flex items-start gap-2">
      <label className="flex flex-1 items-start gap-2 text-body">
        {ready && submitted && <input type="checkbox" className="mt-1" checked={!!item.checked}
          disabled={!canConfirm || busy || !!issue.workbench_pause} onChange={(event) => void saveCheck(item.id, event.target.checked)}/>}
        <span>{item.text}</span>
      </label>
      {onDiscuss && <Button size="xs" variant="ghost" aria-label={t(($) => $.discussion.quote_item, { item: item.id })} onClick={() => {
        useCommentDraftStore.getState().referenceRequirement(`new:${issue.id}`, `${item.id} ${item.text}`);
        onDiscuss();
      }}>{t(($) => $.discussion.quote)}</Button>}
      </div>
      {item.section && previous && document && requirementSection(previous.content, item.section) !== requirementSection(document.content, item.section) && <p className="text-caption text-muted-foreground">{t(($) => $.discussion.item_changed)}</p>}
    </li>)}</ul>
    {!!criteria.bounds.length && <ul className="space-y-1 text-caption text-muted-foreground">{criteria.bounds.map((bound, index) => <li key={index}>{bound}</li>)}</ul>}
    {document && <details className="space-y-3">
      <summary className="cursor-pointer text-label">{t(($) => $.discussion.read_document)}</summary>
      <ReadonlyContent content={document.content}/>
    </details>}
    {document?.prototype && <details data-testid="requirements-prototype" className="space-y-3">
      <summary className="cursor-pointer text-label">{t(($) => $.discussion.prototype)}</summary>
      <HtmlBlockPreview html={document.prototype.content} title={t(($) => $.discussion.prototype)}/>
    </details>}
    {verification && <p role="status" className="text-caption text-muted-foreground">{verification}</p>}
    {document?.error && onDiscuss && <Button size="sm" variant="ghost" onClick={() => {
      useCommentDraftStore.getState().appendToDraftContent(`new:${issue.id}`, t(($) => $.discussion.retry_document));
      onDiscuss();
    }}>{t(($) => $.discussion.retry_document)}</Button>}
    {ready && submitted && canConfirm && <>
      {draftNotice}
      <Button size="sm" disabled={busy || pendingChange || !!issue.workbench_pause || criteria.items.some((item) => !item.checked)} onClick={() => void confirm()}>
        {flow && nextLabel ? t(($) => $.discussion.confirm_start, { stage: nextLabel }) : t(($) => $.requirements.confirm)}
      </Button>
    </>}
    {error && <p role="alert" className="text-caption text-destructive">{error}</p>}
  </section>;
}
