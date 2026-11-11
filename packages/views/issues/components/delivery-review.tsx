"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import type { Issue } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { useActorName } from "@multica/core/workspace/hooks";
import {
  deliveryReviewOptions,
  useReviewDelivery,
  type DeliveryReviewInput,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";
import { ReleaseReadiness } from "./release-readiness";
import { DeliveryRecovery } from "./delivery-recovery";

export function DeliveryReview({ issue }: { issue: Issue }) {
  if (issue.parent_issue_id || !issue.project_id) return null;
  if (!["in_review", "done", "blocked"].includes(issue.status)) return null;
  return <ReviewCard issue={issue} />;
}

function ReviewCard({ issue }: { issue: Issue }) {
  const { t } = useT("issues");
  const { t: projectT } = useT("projects");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const { getActorName } = useActorName();
  const review = useQuery({
    ...deliveryReviewOptions(wsId, issue.id, issue.revision),
    enabled: !issue.acceptance_criteria?.gate.accept,
  });
  const mutation = useReviewDelivery(wsId, issue.id);
  const [verified, setVerified] = useState<{ revision: number; ids: string[] }>();
  const [reason, setReason] = useState<string>();
  const [error, setError] = useState<string>();
  const assigned = issue.assignee_type === "member" && issue.assignee_id === member.userId
    && ["owner", "admin", "member"].includes(member.role ?? "");
  const checked = verified?.revision === review.data?.revision ? verified?.ids ?? [] : [];
  const ready = review.data?.revision === issue.revision && review.data?.blockers.length === 0
    && review.data.checks.every((check) => checked.includes(check.id));

  const act = async (action: DeliveryReviewInput["action"], revision = issue.revision) => {
    if (revision === undefined) return;
    setError(undefined);
    try {
      const after = await mutation.mutateAsync({ expected_revision: revision, action, reason, checks: checked });
      setReason(undefined);
      setVerified(undefined);
      if (action === "accept") {
        toast.success(t(($) => $.delivery_review.accepted), {
          duration: 5000,
          action: { label: t(($) => $.delivery_review.undo), onClick: () => void act("undo", after.revision) },
        });
      }
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : t(($) => $.delivery_review.failed);
      setError(message);
      if (action === "undo") toast.error(message);
    }
  };

  return <section className="mb-6 space-y-3 rounded-xl border bg-surface p-4" aria-label={t(($) => $.delivery_review.title)}>
    <h3 className="text-label font-medium">{t(($) => $.delivery_review.title)}</h3>
    {!issue.acceptance_criteria?.plan && <DeliveryRecovery issue={issue} />}
    {issue.status === "done" && issue.acceptance_criteria?.gate.accept ? (
      <div className="space-y-1 text-caption">
        <p className="text-success">{t(($) => $.delivery_review.accepted)}</p>
        <p className="text-muted-foreground">{t(($) => $.delivery.recorded, {
          actor: getActorName("member", issue.acceptance_criteria.gate.accept.passed_by),
          time: new Date(issue.acceptance_criteria.gate.accept.passed_at).toLocaleString(),
        })}</p>
        <ReleaseReadiness issue={issue} />
      </div>
    ) : <>
      {review.isPending && <p role="status" className="text-caption">{t(($) => $.delivery_review.loading)}</p>}
      {review.isError && <div role="alert" className="space-y-2 text-caption">
        <p>{t(($) => $.delivery_review.failed)}</p>
        <Button size="sm" variant="outline" onClick={() => void review.refetch()}>{t(($) => $.execution_plan.retry)}</Button>
      </div>}
      {review.data && <>
        <ul className="space-y-1 text-caption text-destructive">
          {review.data.blockers.map((blocker, index) => <li key={`${blocker.code}-${blocker.issue_id}-${index}`}>
            {t(($) => $.delivery_review.blockers[blocker.code])}
            {blocker.role && ` · ${projectT(($) => $.roles[blocker.role!])}`}
            {blocker.title && <span className="ml-1 break-words text-muted-foreground">{blocker.title}</span>}
          </li>)}
        </ul>
        {review.data.checks.map((check) => <label key={check.id} className="flex items-start gap-2 text-caption">
          <input type="checkbox" disabled={!assigned || mutation.isPending} checked={checked.includes(check.id)} onChange={(event) => {
            setVerified({
              revision: review.data!.revision,
              ids: event.target.checked ? [...checked, check.id] : checked.filter((id) => id !== check.id),
            });
          }} />
          <span>{check.text}{check.check && <span className="block text-muted-foreground">{check.check}</span>}</span>
        </label>)}
        {assigned && issue.status === "in_review" ? <>
          {reason !== undefined && <label className="block space-y-1 text-caption">
            {t(($) => $.delivery_review.reason)}
            <Textarea value={reason} maxLength={2000} onChange={(event) => setReason(event.target.value)} />
          </label>}
          <div className="flex flex-wrap justify-end gap-2">
            {reason === undefined ? (
              <Button variant="outline" disabled={mutation.isPending} onClick={() => setReason("")}>{t(($) => $.delivery_review.request_changes)}</Button>
            ) : <>
              <Button variant="outline" disabled={mutation.isPending} onClick={() => setReason(undefined)}>{t(($) => $.delivery.cancel)}</Button>
              <Button variant="outline" disabled={!reason.trim() || mutation.isPending} onClick={() => void act("reject")}>{t(($) => $.delivery_review.submit_changes)}</Button>
            </>}
            <Button disabled={!ready || mutation.isPending} aria-busy={mutation.isPending} onClick={() => void act("accept")}>{t(($) => $.delivery_review.accept)}</Button>
          </div>
        </> : <div className="space-y-2 text-caption text-muted-foreground">
          <p>{t(($) => $.delivery_review.waiting)}</p>
          {issue.acceptance_criteria?.plan && ["in_review", "done"].includes(issue.status)
            && (issue.assignee_type !== "member" || !issue.assignee_id || assigned)
            && ["owner", "admin", "member"].includes(member.role ?? "") && (
            <Button variant="outline" disabled={mutation.isPending} onClick={() => void act("claim")}>{t(($) => $.delivery_review.claim)}</Button>
          )}
        </div>}
      </>}
    </>}
    {error && <p role="alert" className="text-caption text-destructive">{error}</p>}
  </section>;
}
