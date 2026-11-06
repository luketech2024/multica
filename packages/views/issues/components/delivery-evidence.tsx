"use client";

import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { useActorName } from "@multica/core/workspace/hooks";
import { childIssuesOptions } from "@multica/core/issues/queries";
import type { Issue } from "@multica/core/types";
import {
  useSaveDeliveryEvidence,
  deliveryReviewOptions,
  type DeliveryEvidenceInput,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Dialog, DialogContent, DialogDescription,
  DialogHeader, DialogTitle, DialogTrigger,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n";

export function DeliveryEvidence({ issue }: { issue: Issue }) {
  if (issue.parent_issue_id || !issue.acceptance_criteria?.plan) return null;
  return <DeliveryEvidenceContent issue={issue} />;
}

function DeliveryEvidenceContent({ issue }: { issue: Issue }) {
  const { t } = useT("issues");
  const { t: projectT } = useT("projects");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const { getActorName } = useActorName();
  const children = useQuery(childIssuesOptions(wsId, issue.id));
  const criteria = issue.acceptance_criteria!;
  const verification = useQuery({
    ...deliveryReviewOptions(wsId, issue.id, issue.revision),
    enabled: Object.keys(criteria.delivery ?? {}).length > 0,
  });
  const planned = (children.data ?? []).filter((child) =>
    criteria.plan!.issues.includes(child.id),
  );
  const editable = ["owner", "admin", "member"].includes(member.role ?? "")
    && ["in_progress", "in_review", "blocked"].includes(issue.status)
    && !criteria.change && !criteria.needs_resplit && criteria.plan?.version === criteria.version;
  return <section
    className="space-y-3 rounded-xl border bg-surface p-3"
    aria-label={t(($) => $.delivery.title)}
  >
    <h3 className="text-label font-medium">{t(($) => $.delivery.title)}</h3>
    {children.isPending && <p role="status" className="text-caption">{t(($) => $.pipeline.loading)}</p>}
    {children.isError && <div role="alert" className="space-y-2 text-caption">
      <p>{t(($) => $.pipeline.failed)}</p>
      <Button variant="outline" size="sm" onClick={() => void children.refetch()}>
        {t(($) => $.execution_plan.retry)}
      </Button>
    </div>}
    {!children.isPending && !children.isError && planned.length !== criteria.plan!.issues.length && (
      <p role="alert" className="text-caption text-destructive">{t(($) => $.delivery.missing_tasks)}</p>
    )}
    {planned.map((child) => {
      const evidence = criteria.delivery?.[child.id];
      const handoff = child.workbench_handoff;
      const current = !!evidence && evidence.issue_revision === child.revision && evidence.version === criteria.version
        && handoff?.requirement_version === criteria.version && !handoff.superseded && child.status === "done"
        && !verification.data?.blockers.some((blocker) => blocker.code === "plan" || (blocker.issue_id === child.id && blocker.code === "evidence_stale"));
      const verified = verification.isSuccess && verification.data.revision === issue.revision;
      const result = !verified ? "unverified" : !current ? "stale" : evidence?.passed ? "passed" : "failed";
      return <div key={child.id} className="space-y-2 border-t pt-3 text-caption">
        <p className="font-medium break-words">{child.identifier} · {child.title}</p>
        {handoff && <p className="text-muted-foreground">{projectT(($) => $.roles[handoff.role])}</p>}
        {evidence ? <>
          <p className={result === "passed" ? "text-success" : result === "unverified" ? "text-muted-foreground" : "text-destructive"}>
            {t(($) => $.delivery[result])}
          </p>
          <p className="whitespace-pre-wrap break-words">{evidence.summary}</p>
          <p className="break-words text-muted-foreground">
            {t(($) => $.delivery.recorded, {
              actor: getActorName(evidence.actor_type, evidence.recorded_by),
              time: new Date(evidence.recorded_at).toLocaleString(),
            })}
          </p>
          {evidence.commit && <p className="break-all font-mono">{evidence.commit}</p>}
          {evidence.changes && <p>{t(($) => $.delivery.statistics, evidence.changes)}</p>}
          <ul className="space-y-1">
            {evidence.reports.map((report) => <li key={report.name}>
              <a className="break-words text-primary hover:underline" href={report.url} target="_blank" rel="noopener noreferrer">
                {report.name}
              </a>
            </li>)}
          </ul>
          {evidence.preview_url && (
            <a className="block text-primary hover:underline" href={evidence.preview_url} target="_blank" rel="noopener noreferrer">
              {t(($) => $.delivery.preview)}
            </a>
          )}
        </> : <p className="text-muted-foreground">{t(($) => $.delivery.empty)}</p>}
        {editable && handoff && !handoff.superseded && handoff.requirement_version === criteria.version && child.status === "done" && (
          <EvidenceEditor issue={issue} child={child} />
        )}
      </div>;
    })}
  </section>;
}

function EvidenceEditor({ issue, child }: { issue: Issue; child: Issue }) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const save = useSaveDeliveryEvidence(wsId, issue.id);
  const [draft, setDraft] = useState<{
    revision: number;
    evidence: DeliveryEvidenceInput;
  }>();
  const [error, setError] = useState<string>();
  const handoff = child.workbench_handoff!;
  const open = (value: boolean) => {
    if (save.isPending) return;
    setError(undefined);
    if (!value) {
      setDraft(undefined);
      return;
    }
    const previous = issue.acceptance_criteria?.delivery?.[child.id];
    const expected = handoff.expected.length ? handoff.expected : [t(($) => $.delivery.report)];
    setDraft({
      revision: issue.revision!,
      evidence: {
        issue_id: child.id,
        issue_revision: child.revision!,
        commit: previous?.commit ?? "",
        passed: false,
        summary: previous?.summary ?? "",
        reports: expected.map((name) => ({
          name,
          url: previous?.reports.find((report) => report.name === name)?.url ?? "",
        })),
        changes: handoff.role === "dev" ? previous?.changes ?? { files: 0, additions: 0, deletions: 0 } : null,
        preview_url: previous?.preview_url ?? "",
      },
    });
  };
  const edit = (patch: Partial<DeliveryEvidenceInput>) => {
    if (draft) setDraft({ ...draft, evidence: { ...draft.evidence, ...patch } });
  };
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!draft) return;
    setError(undefined);
    try {
      await save.mutateAsync(draft);
      setDraft(undefined);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t(($) => $.delivery.save_failed));
    }
  };
  return <Dialog open={!!draft} onOpenChange={open}>
    <DialogTrigger render={<Button variant="outline" size="sm" disabled={issue.revision === undefined || child.revision === undefined} />}>
      {t(($) => $.delivery.record)}
    </DialogTrigger>
    <DialogContent className="max-h-[85dvh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{t(($) => $.delivery.record)} · {child.identifier}</DialogTitle>
        <DialogDescription>{t(($) => $.delivery.attribution)}</DialogDescription>
      </DialogHeader>
      {draft && <form onSubmit={submit}>
        <fieldset disabled={save.isPending} className="space-y-3">
        <label className="block space-y-1 text-caption">
          {t(($) => $.delivery.conclusion)}
          <Textarea required maxLength={2000} value={draft.evidence.summary} onChange={(event) => edit({ summary: event.target.value })} />
        </label>
        {handoff.repo_id && <label className="block space-y-1 text-caption">
          {t(($) => $.delivery.commit)}
          <Input required pattern="[a-fA-F0-9]{40}([a-fA-F0-9]{24})?" value={draft.evidence.commit} onChange={(event) => edit({ commit: event.target.value })} />
        </label>}
        {draft.evidence.reports.map((report, index) => <label key={report.name} className="block space-y-1 text-caption">
          {report.name}
          <Input type="url" required value={report.url} onChange={(event) => edit({
            reports: draft.evidence.reports.map((entry, i) => i === index ? { ...entry, url: event.target.value } : entry),
          })} />
        </label>)}
        {draft.evidence.changes && <div className="grid grid-cols-3 gap-2">
          {(["files", "additions", "deletions"] as const).map((field) => <label key={field} className="space-y-1 text-caption">
            {t(($) => $.delivery[field])}
            <Input type="number" required min={0} step={1} value={draft.evidence.changes![field]} onChange={(event) => edit({
              changes: { ...draft.evidence.changes!, [field]: Number(event.target.value) },
            })} />
          </label>)}
        </div>}
        {handoff.role === "dev" && <label className="block space-y-1 text-caption">
          {t(($) => $.delivery.preview)}
          <Input type="url" required={draft.evidence.passed} value={draft.evidence.preview_url} onChange={(event) => edit({ preview_url: event.target.value })} />
        </label>}
        <label className="flex items-start gap-2 text-caption">
          <input type="checkbox" checked={draft.evidence.passed} onChange={(event) => edit({ passed: event.target.checked })} />
          {t(($) => $.delivery.confirm_pass)}
        </label>
        {error && <p role="alert" className="text-caption text-destructive">{error}</p>}
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" disabled={save.isPending} onClick={() => open(false)}>{t(($) => $.delivery.cancel)}</Button>
          <Button type="submit" disabled={save.isPending} aria-busy={save.isPending}>{t(($) => $.delivery.save)}</Button>
        </div>
        </fieldset>
      </form>}
    </DialogContent>
  </Dialog>;
}
